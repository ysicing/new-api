package service

import (
	"context"
	"fmt"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/alicebob/miniredis/v2"
	"github.com/gin-gonic/gin"
	"github.com/go-redis/redis/v8"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupChannelConcurrencyStore(t *testing.T, distributed bool) {
	t.Helper()
	previousRedis, previousRDB := common.RedisEnabled, common.RDB
	common.RedisEnabled = distributed
	if distributed {
		server := miniredis.RunT(t)
		client := redis.NewClient(&redis.Options{Addr: server.Addr()})
		common.RDB = client
		t.Cleanup(func() { _ = client.Close() })
	}
	t.Cleanup(func() { common.RedisEnabled, common.RDB = previousRedis, previousRDB })
}

func TestChannelConcurrencyAtomicLimitAndRelease(t *testing.T) {
	for _, distributed := range []bool{false, true} {
		t.Run(fmt.Sprint(distributed), func(t *testing.T) {
			setupChannelConcurrencyStore(t, distributed)
			start := make(chan struct{})
			type result struct {
				release func()
				err     error
			}
			results := make(chan result, 8)
			var workers sync.WaitGroup
			for i := 0; i < 8; i++ {
				workers.Add(1)
				go func() {
					defer workers.Done()
					<-start
					release, err := acquireChannelSlot(context.Background(), 101, 3, func() {})
					results <- result{release, err}
				}()
			}
			close(start)
			workers.Wait()
			close(results)
			var releases []func()
			for result := range results {
				if result.err == nil {
					releases = append(releases, result.release)
				} else {
					assert.ErrorIs(t, result.err, ErrChannelConcurrencyFull)
				}
			}
			assert.Len(t, releases, 3)
			for _, release := range releases {
				release()
				release()
			}
			release, err := acquireChannelSlot(context.Background(), 101, 1, func() {})
			require.NoError(t, err)
			release()
		})
	}
}

func TestRedisChannelLeaseRenewsAndExpiresByOwner(t *testing.T) {
	setupChannelConcurrencyStore(t, true)
	ctx := context.Background()
	key := "channel:concurrency:202"
	require.NoError(t, common.RDB.ZAdd(ctx, key, &redis.Z{Score: 1, Member: "expired"}).Err())
	acquired, err := acquireChannelLease.Run(ctx, common.RDB, []string{key}, 1, "owner", channelLeaseTTL.Milliseconds()).Int()
	require.NoError(t, err)
	assert.Equal(t, 1, acquired)
	renewed, err := renewChannelLease.Run(ctx, common.RDB, []string{key}, "owner", channelLeaseTTL.Milliseconds()).Int()
	require.NoError(t, err)
	assert.Equal(t, 1, renewed)
	renewed, err = renewChannelLease.Run(ctx, common.RDB, []string{key}, "other", channelLeaseTTL.Milliseconds()).Int()
	require.NoError(t, err)
	assert.Zero(t, renewed)
	acquired, err = acquireChannelLease.Run(ctx, common.RDB, []string{key}, 1, "other", channelLeaseTTL.Milliseconds()).Int()
	require.NoError(t, err)
	assert.Zero(t, acquired)
}

func concurrencyRequest(t *testing.T) *gin.Context {
	t.Helper()
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest("POST", "/v1/chat/completions", nil)
	t.Cleanup(func() { ReleaseChannelConcurrency(c) })
	return c
}

func TestChannelConcurrencyUnlimitedAndCanceledRequests(t *testing.T) {
	setupChannelConcurrencyStore(t, false)
	for _, limit := range []*int{nil, common.GetPointer(0)} {
		c := concurrencyRequest(t)
		require.NoError(t, reserveChannelConcurrency(c, &model.Channel{Id: 303, MaxConcurrency: limit}))
		_, exists := c.Get(channelLeaseKey)
		assert.False(t, exists)
	}
	c := concurrencyRequest(t)
	ctx, cancel := context.WithCancel(c.Request.Context())
	c.Request = c.Request.WithContext(ctx)
	channel := &model.Channel{Id: 303, MaxConcurrency: common.GetPointer(1)}
	require.NoError(t, reserveChannelConcurrency(c, channel))
	require.NoError(t, reserveChannelConcurrency(c, channel)) // 同一请求不重复占名额
	other := concurrencyRequest(t)
	require.ErrorIs(t, reserveChannelConcurrency(other, channel), ErrChannelConcurrencyFull)
	cancel()
	ReleaseChannelConcurrency(c)
	require.NoError(t, reserveChannelConcurrency(other, channel))
}

func TestChannelConcurrencyRedisFailureDoesNotFallBackToLocal(t *testing.T) {
	setupChannelConcurrencyStore(t, true)
	require.NoError(t, common.RDB.Close())
	channel := &model.Channel{Id: 404, MaxConcurrency: common.GetPointer(1)}
	_, apiErr := ReserveChannelForRequest(concurrencyRequest(t), channel)
	require.NotNil(t, apiErr)
	assert.Equal(t, 503, apiErr.StatusCode)
}

func TestChannelConcurrencyDoesNotRerouteFullChannel(t *testing.T) {
	setupChannelConcurrencyStore(t, false)
	primary := &model.Channel{Id: 501, MaxConcurrency: common.GetPointer(1)}
	backup := &model.Channel{Id: 502}
	occupying := concurrencyRequest(t)
	require.NoError(t, reserveChannelConcurrency(occupying, primary))
	request := concurrencyRequest(t)
	request.Set("channel_id", primary.Id)
	selected, apiErr := ReserveChannelForRequest(request, primary)
	require.Nil(t, selected)
	require.NotNil(t, apiErr)
	assert.Equal(t, 429, apiErr.StatusCode)
	assert.Equal(t, primary.Id, request.GetInt("channel_id"))
	// 另一个渠道即使空闲，也不因当前渠道满载而被本功能自动选中。
	_, apiErr = ReserveChannelForRequest(concurrencyRequest(t), backup)
	require.Nil(t, apiErr)
}

func TestMidjourneyFinalSendRejectsFullChannelWithoutEarlierReservation(t *testing.T) {
	setupChannelConcurrencyStore(t, false)
	db := setupChannelSelectAutoGroupsTest(t)
	createChannelSelectAutoGroupsChannel(t, db, 1001, "default", "mj_video")
	require.NoError(t, db.Model(&model.Channel{}).Where("id = ?", 1001).Update("max_concurrency", 1).Error)
	model.InitChannelCache()
	channel, err := model.CacheGetChannel(1001)
	require.NoError(t, err)
	occupying := concurrencyRequest(t)
	require.NoError(t, reserveChannelConcurrency(occupying, channel))
	request := concurrencyRequest(t)
	request.Set("channel_id", 1001)
	result, _, err := DoMidjourneyHttpRequest(request, time.Second, "http://unused.invalid/mj/submit/video")
	require.ErrorIs(t, err, ErrChannelConcurrencyFull)
	require.NotNil(t, result)
	assert.Equal(t, 429, result.StatusCode)
	assert.Equal(t, 30, result.Response.Code)
}
