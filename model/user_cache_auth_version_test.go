package model

import (
	"errors"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/alicebob/miniredis/v2"
	"github.com/go-redis/redis/v8"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// Sharded Redis hashes only the text inside the first brace pair, so fence
// and version keys tagged with the untagged cache key share its slot.
// Miniredis does not enforce slots, so check the tags explicitly.
func TestAuthCacheKeysShareClusterHashTag(t *testing.T) {
	for _, tc := range []struct {
		cacheKey string
		related  []string
	}{
		{getUserCacheKey(5), []string{getUserAuthFenceKey(5), getUserAuthVersionKey(5), getUserQuotaFreezeRevisionKey(5)}},
		{getTokenCacheKey("test-token"), []string{getTokenCacheFenceKey("test-token")}},
	} {
		assert.NotContains(t, tc.cacheKey, "{")
		for _, key := range tc.related {
			_, suffix, ok := strings.Cut(key, "{")
			require.True(t, ok)
			tag, _, ok := strings.Cut(suffix, "}")
			require.True(t, ok)
			assert.Equal(t, tc.cacheKey, tag)
		}
	}
}

func useUserCacheMiniRedis(t *testing.T) *miniredis.Miniredis {
	t.Helper()
	server := miniredis.RunT(t)
	oldRedisEnabled := common.RedisEnabled
	oldRDB := common.RDB
	oldSyncFrequency := common.SyncFrequency
	common.RedisEnabled = true
	common.SyncFrequency = 2
	common.RDB = redis.NewClient(&redis.Options{Addr: server.Addr()})
	t.Cleanup(func() {
		_ = common.RDB.Close()
		common.RedisEnabled = oldRedisEnabled
		common.RDB = oldRDB
		common.SyncFrequency = oldSyncFrequency
	})
	return server
}

func TestQuotaFreezePublishesToCachedUser(t *testing.T) {
	truncateTables(t)
	useUserCacheMiniRedis(t)
	user := User{Username: "quota-freeze-cache", Password: "password", AffCode: "quota-freeze-cache", Status: common.UserStatusEnabled, AuthVersion: 1}
	require.NoError(t, DB.Create(&user).Error)
	require.NoError(t, populateUserCache(user))

	require.NoError(t, SetUserQuotaFrozen(user.Id, true))
	require.NoError(t, updateUserCache(user)) // A delayed pre-freeze snapshot must not undo the restriction.
	cached, err := GetUserCache(user.Id)
	require.NoError(t, err)
	assert.True(t, cached.QuotaFrozen)
	assert.Equal(t, common.UserStatusEnabled, cached.Status)
	assert.EqualValues(t, 1, cached.AuthVersion)

	require.NoError(t, SetUserQuotaFrozen(user.Id, false))
	staleFrozen := user
	staleFrozen.QuotaFrozen = true
	require.NoError(t, updateUserCache(staleFrozen))
	cached, err = GetUserCache(user.Id)
	require.NoError(t, err)
	assert.False(t, cached.QuotaFrozen)
	assert.EqualValues(t, 1, cached.AuthVersion)
}

func TestQuotaFreezeCacheRepairsTamperingWithoutLosingReservedQuota(t *testing.T) {
	truncateTables(t)
	useUserCacheMiniRedis(t)
	user := User{Username: "freeze-cache-repair", AffCode: "freeze-repair", Status: common.UserStatusEnabled, AuthVersion: 1, Quota: 100}
	require.NoError(t, DB.Create(&user).Error)
	require.NoError(t, populateUserCache(user))
	require.NoError(t, cacheIncrUserQuota(user.Id, -70))
	session := newTestUserSession("freeze-preserves-session", user.Id, time.Now().Unix())
	require.NoError(t, CreateUserSession(session))

	for _, frozen := range []bool{true, false} {
		require.NoError(t, SetUserQuotaFrozen(user.Id, frozen))
		require.NoError(t, common.RDB.HSet(t.Context(), getUserCacheKey(user.Id), "QuotaFrozen", !frozen).Err())
		_, err := cacheGetUserBase(user.Id)
		require.Error(t, err)
		cached, err := GetUserCache(user.Id)
		require.NoError(t, err)
		assert.Equal(t, frozen, cached.QuotaFrozen)
		cached, err = cacheGetUserBase(user.Id)
		require.NoError(t, err, "a DB fallback must repair the signed cache")
		assert.Equal(t, frozen, cached.QuotaFrozen)
		assert.Equal(t, 30, cached.Quota)
		assert.EqualValues(t, 1, cached.AuthVersion)
		active, err := GetUserSessionCached(session.SID)
		require.NoError(t, err)
		assert.Equal(t, UserSessionStatusActive, active.Status)
	}
}

func TestQuotaFreezeFencesConcurrentCacheFillAfterHashExpiry(t *testing.T) {
	for _, frozen := range []bool{true, false} {
		t.Run(map[bool]string{true: "freeze", false: "unfreeze"}[frozen], func(t *testing.T) {
			truncateTables(t)
			server := useUserCacheMiniRedis(t)
			user := User{Username: "freeze-racing-fill", AffCode: "freeze-race", AuthVersion: 1, QuotaFrozen: !frozen}
			require.NoError(t, DB.Create(&user).Error)
			require.NoError(t, populateUserCache(user))

			snapshotRead := make(chan struct{})
			releaseFill := make(chan struct{})
			defer close(releaseFill)
			var intercepted atomic.Bool
			const callbackName = "test:block_freeze_snapshot"
			require.NoError(t, DB.Callback().Query().After("gorm:query").Register(callbackName, func(tx *gorm.DB) {
				if tx.Statement.Table == "users" && len(tx.Statement.Selects) == 1 &&
					tx.Statement.Selects[0] == "quota_frozen" && intercepted.CompareAndSwap(false, true) {
					close(snapshotRead)
					<-releaseFill
				}
			}))
			t.Cleanup(func() { assert.NoError(t, DB.Callback().Query().Remove(callbackName)) })
			done := make(chan error, 1)
			go func() { done <- populateUserCache(user) }()
			<-snapshotRead
			require.NoError(t, SetUserQuotaFrozen(user.Id, frozen))
			server.FastForward(time.Duration(userCacheTTLSeconds()+1) * time.Second)
			assert.False(t, server.Exists(getUserCacheKey(user.Id)))
			assert.True(t, server.Exists(getUserQuotaFreezeRevisionKey(user.Id)))
			releaseFill <- struct{}{}
			require.NoError(t, <-done)
			cached, err := cacheGetUserBase(user.Id)
			require.NoError(t, err)
			assert.Equal(t, frozen, cached.QuotaFrozen)
			assert.EqualValues(t, 1, cached.AuthVersion)
		})
	}
}

func TestQuotaFreezeConcurrentUpdatesPublishDatabaseOrder(t *testing.T) {
	truncateTables(t)
	useUserCacheMiniRedis(t)
	user := User{Username: "freeze-racing-updates", AffCode: "freeze-toggle", AuthVersion: 1}
	require.NoError(t, DB.Create(&user).Error)
	require.NoError(t, populateUserCache(user))
	firstCommitted := make(chan struct{})
	releaseFirst := make(chan struct{})
	defer close(releaseFirst)
	var intercepted atomic.Bool
	const callbackName = "test:block_freeze_publication"
	require.NoError(t, DB.Callback().Update().After("gorm:commit_or_rollback_transaction").Register(callbackName, func(tx *gorm.DB) {
		if tx.Statement.Table == "users" && intercepted.CompareAndSwap(false, true) {
			close(firstCommitted)
			<-releaseFirst
		}
	}))
	t.Cleanup(func() { assert.NoError(t, DB.Callback().Update().Remove(callbackName)) })
	done := make(chan error, 1)
	go func() { done <- SetUserQuotaFrozen(user.Id, true) }()
	<-firstCommitted
	require.NoError(t, SetUserQuotaFrozen(user.Id, false))
	cached, err := GetUserCache(user.Id)
	require.NoError(t, err)
	assert.False(t, cached.QuotaFrozen)
	releaseFirst <- struct{}{}
	require.NoError(t, <-done)
	cached, err = GetUserCache(user.Id)
	require.NoError(t, err)
	assert.False(t, cached.QuotaFrozen)
	require.NoError(t, updateUserCache(User{Id: user.Id, AuthVersion: 1, QuotaFrozen: true}))
	cached, err = cacheGetUserBase(user.Id)
	require.NoError(t, err)
	assert.False(t, cached.QuotaFrozen)
	assert.EqualValues(t, 1, cached.AuthVersion)
}

func TestUserAuthFenceRollbackExpiresAndRecovers(t *testing.T) {
	truncateTables(t)
	server := useUserCacheMiniRedis(t)

	user := User{
		Username:    "auth-fence-rollback",
		Password:    "password",
		Role:        common.RoleCommonUser,
		Status:      common.UserStatusEnabled,
		Group:       "default",
		AuthVersion: 1,
	}
	require.NoError(t, DB.Create(&user).Error)
	require.NoError(t, populateUserCache(user))

	tx := DB.Begin()
	require.NoError(t, tx.Error)
	next, err := IncrementUserAuthVersionWithTx(tx, user.Id)
	require.NoError(t, err)
	assert.EqualValues(t, 2, next)

	_, err = cacheGetUserBase(user.Id)
	assert.ErrorIs(t, err, ErrUserAuthCachePending)
	cacheTTL, err := common.RDB.TTL(t.Context(), getUserCacheKey(user.Id)).Result()
	require.NoError(t, err)
	fenceTTL, err := common.RDB.TTL(t.Context(), getUserAuthFenceKey(user.Id)).Result()
	require.NoError(t, err)
	assert.Greater(t, fenceTTL, cacheTTL)
	require.NoError(t, tx.Rollback().Error)

	server.FastForward(time.Duration(userAuthFenceTTLSeconds()+1) * time.Second)
	assert.False(t, server.Exists(getUserAuthFenceKey(user.Id)))
	committed, err := common.RDB.Get(t.Context(), getUserAuthVersionKey(user.Id)).Result()
	require.NoError(t, err)
	assert.Equal(t, "1", committed)

	cached, err := GetUserCache(user.Id)
	require.NoError(t, err)
	assert.EqualValues(t, 1, cached.AuthVersion)
}

func TestPendingUserAuthFenceRejectsStaleCacheWrite(t *testing.T) {
	truncateTables(t)
	server := useUserCacheMiniRedis(t)
	const userID = 4201
	require.NoError(t, DB.Create(&User{Id: userID, Username: "pending-fill", AffCode: "pending-fill", AuthVersion: 1}).Error)
	require.NoError(t, SetUserAuthVersionFence(userID, 2))

	err := writeUserCache(&UserBase{
		Id: userID, Group: "default", Username: "stale", AuthVersion: 1,
	}, true)

	assert.ErrorIs(t, err, ErrUserAuthCachePending)
	assert.False(t, server.Exists(getUserCacheKey(userID)))
}

func TestUserAuthCacheUpdateRejectsVersionMismatch(t *testing.T) {
	truncateTables(t)
	useUserCacheMiniRedis(t)
	const userID = 4202
	require.NoError(t, DB.Create(&User{Id: userID, Username: "version-fill", AffCode: "version-fill", AuthVersion: 3}).Error)
	require.NoError(t, writeUserCache(&UserBase{
		Id: userID, Group: "current", Username: "cached", AuthVersion: 3,
	}, true))

	err := writeUserCache(&UserBase{Id: userID, Group: "stale", AuthVersion: 2}, false)

	assert.ErrorIs(t, err, ErrUserAuthCachePending)
	group, err := common.RDB.HGet(t.Context(), getUserCacheKey(userID), "Group").Result()
	require.NoError(t, err)
	assert.Equal(t, "current", group)
}

func TestRefreshUserGroupCacheRepairsDelayedSameVersionWrite(t *testing.T) {
	truncateTables(t)
	useUserCacheMiniRedis(t)

	user := User{
		Username:    "delayed-group-refresh",
		Password:    "password",
		Role:        common.RoleCommonUser,
		Status:      common.UserStatusEnabled,
		Group:       "default",
		AuthVersion: 1,
	}
	require.NoError(t, DB.Create(&user).Error)
	require.NoError(t, populateUserCache(user))

	firstSnapshotRead := make(chan struct{})
	releaseDelayedRefresh := make(chan struct{})
	var intercepted atomic.Bool
	const callbackName = "test:block_delayed_group_refresh"
	require.NoError(t, DB.Callback().Query().After("gorm:query").Register(callbackName, func(*gorm.DB) {
		if intercepted.CompareAndSwap(false, true) {
			close(firstSnapshotRead)
			<-releaseDelayedRefresh
		}
	}))
	t.Cleanup(func() {
		_ = DB.Callback().Query().Remove(callbackName)
	})

	delayedResult := make(chan error, 1)
	go func() {
		delayedResult <- RefreshUserGroupCache(user.Id)
	}()
	<-firstSnapshotRead

	require.NoError(t, DB.Model(&User{}).Where("id = ?", user.Id).Update("group", "pro").Error)
	require.NoError(t, RefreshUserGroupCache(user.Id))
	cached, err := cacheGetUserBase(user.Id)
	require.NoError(t, err)
	assert.Equal(t, "pro", cached.Group)
	assert.EqualValues(t, 1, cached.AuthVersion)

	close(releaseDelayedRefresh)
	require.NoError(t, <-delayedResult)
	cached, err = cacheGetUserBase(user.Id)
	require.NoError(t, err)
	assert.Equal(t, "pro", cached.Group)
	assert.EqualValues(t, 1, cached.AuthVersion)
}

func TestCommittedUserAuthVersionPermanentlyRejectsDelayedCacheFill(t *testing.T) {
	truncateTables(t)
	server := useUserCacheMiniRedis(t)

	user := User{
		Username:    "auth-fence-commit",
		Password:    "password",
		Role:        common.RoleCommonUser,
		Status:      common.UserStatusEnabled,
		Group:       "default",
		AuthVersion: 1,
	}
	require.NoError(t, DB.Create(&user).Error)
	require.NoError(t, populateUserCache(user))
	stale := *user.ToBaseUser()

	require.NoError(t, DB.Transaction(func(tx *gorm.DB) error {
		_, err := IncrementUserAuthVersionWithTx(tx, user.Id)
		return err
	}))
	require.NoError(t, PublishUserAuthCache(user.Id))
	assert.False(t, server.Exists(getUserAuthFenceKey(user.Id)))
	committed, err := common.RDB.Get(t.Context(), getUserAuthVersionKey(user.Id)).Result()
	require.NoError(t, err)
	assert.Equal(t, "2", committed)

	server.FastForward(time.Duration(userAuthFenceTTLSeconds()+1) * time.Second)
	require.NoError(t, common.RedisDelKey(getUserCacheKey(user.Id)))
	err = writeUserCache(&stale, true)
	assert.True(t, errors.Is(err, ErrUserAuthCachePending))
	committed, err = common.RDB.Get(t.Context(), getUserAuthVersionKey(user.Id)).Result()
	require.NoError(t, err)
	assert.Equal(t, "2", committed)
}

func TestUserAuthVersionFenceAndCommittedFloorAreMonotonic(t *testing.T) {
	truncateTables(t)
	server := useUserCacheMiniRedis(t)

	const userID = 4101
	require.NoError(t, SetUserAuthVersionFence(userID, 5))
	require.NoError(t, SetUserAuthVersionFence(userID, 3))
	pending, err := common.RDB.Get(t.Context(), getUserAuthFenceKey(userID)).Result()
	require.NoError(t, err)
	assert.Equal(t, "5", pending)
	floor, err := getUserAuthVersionFloor(userID)
	require.NoError(t, err)
	assert.EqualValues(t, 5, floor)

	// Committing an older transaction must neither clear a newer pending fence
	// nor lower the effective deny floor.
	require.NoError(t, publishCommittedUserAuthVersion(userID, 3))
	pending, err = common.RDB.Get(t.Context(), getUserAuthFenceKey(userID)).Result()
	require.NoError(t, err)
	assert.Equal(t, "5", pending)
	floor, err = getUserAuthVersionFloor(userID)
	require.NoError(t, err)
	assert.EqualValues(t, 5, floor)

	require.NoError(t, publishCommittedUserAuthVersion(userID, 5))
	assert.False(t, server.Exists(getUserAuthFenceKey(userID)))
	committed, err := common.RDB.Get(t.Context(), getUserAuthVersionKey(userID)).Result()
	require.NoError(t, err)
	assert.Equal(t, "5", committed)

	require.NoError(t, publishCommittedUserAuthVersion(userID, 4))
	committed, err = common.RDB.Get(t.Context(), getUserAuthVersionKey(userID)).Result()
	require.NoError(t, err)
	assert.Equal(t, "5", committed)
}
