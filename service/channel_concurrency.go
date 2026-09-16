package service

import (
	"context"
	"errors"
	"fmt"
	"math/rand/v2"
	"net/http"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/relaykit/types"
	"github.com/gin-gonic/gin"
	"github.com/go-redis/redis/v8"
	"github.com/google/uuid"
)

var ErrChannelConcurrencyFull = errors.New(channelConcurrencyLimitMessage)

const channelLeaseTTL = 30 * time.Second
const channelLeaseKey = "channel_concurrency_lease"
const channelConcurrencyRejectedKey = "channel_concurrency_rejected"
const channelConcurrencyLimitMessage = "Concurrency limit exceeded for account, please retry later"

// 每个请求使用独立租约。Redis 服务端时间避免实例时钟偏移；续租支持长流，过期回收崩溃实例的名额。
var acquireChannelLease = redis.NewScript(`
local t = redis.call('TIME')
local now = t[1] * 1000 + math.floor(t[2] / 1000)
redis.call('ZREMRANGEBYSCORE', KEYS[1], '-inf', now)
if redis.call('ZCARD', KEYS[1]) >= tonumber(ARGV[1]) then return 0 end
redis.call('ZADD', KEYS[1], now + tonumber(ARGV[3]), ARGV[2])
redis.call('PEXPIRE', KEYS[1], ARGV[3])
return 1`)
var renewChannelLease = redis.NewScript(`
local t = redis.call('TIME')
local now = t[1] * 1000 + math.floor(t[2] / 1000)
local expiry = redis.call('ZSCORE', KEYS[1], ARGV[1])
if not expiry or tonumber(expiry) <= now then return 0 end
redis.call('ZADD', KEYS[1], now + tonumber(ARGV[2]), ARGV[1])
redis.call('PEXPIRE', KEYS[1], ARGV[2])
return 1`)

var localChannelSlots = struct {
	sync.Mutex
	counts map[int]int
}{counts: make(map[int]int)}

type channelRequestLease struct {
	channelID int
	original  context.Context
	release   func()
}

// ReleaseChannelConcurrency 在请求结束或切换渠道时释放名额；不受已取消的请求上下文影响。
func ReleaseChannelConcurrency(c *gin.Context) {
	if value, ok := c.Get(channelLeaseKey); ok && value != nil {
		lease := value.(*channelRequestLease)
		c.Set(channelLeaseKey, nil)
		c.Request = c.Request.WithContext(lease.original)
		lease.release()
	}
}

// ChannelConcurrencyContext 返回当前租约的取消上下文；无限制渠道返回 nil，保留原有传输行为。
func ChannelConcurrencyContext(c *gin.Context) context.Context {
	if value, ok := c.Get(channelLeaseKey); ok && value != nil {
		return c.Request.Context()
	}
	return nil
}

func reserveChannelConcurrency(c *gin.Context, channel *model.Channel) error {
	if value, ok := c.Get(channelLeaseKey); ok && value != nil && value.(*channelRequestLease).channelID == channel.Id {
		return nil
	}
	ReleaseChannelConcurrency(c)
	if channel.MaxConcurrency == nil || *channel.MaxConcurrency == 0 {
		return nil
	}
	original := c.Request.Context()
	ctx, cancel := context.WithCancel(original)
	release, err := acquireChannelSlot(ctx, channel.Id, *channel.MaxConcurrency, cancel)
	if err != nil {
		cancel()
		return err
	}
	c.Request = c.Request.WithContext(ctx)
	c.Set(channelLeaseKey, &channelRequestLease{channelID: channel.Id, original: original, release: func() { release(); cancel() }})
	return nil
}

func acquireChannelSlot(ctx context.Context, channelID, limit int, cancel context.CancelFunc) (func(), error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if !common.RedisEnabled {
		localChannelSlots.Lock()
		if localChannelSlots.counts[channelID] >= limit {
			localChannelSlots.Unlock()
			return nil, ErrChannelConcurrencyFull
		}
		localChannelSlots.counts[channelID]++
		localChannelSlots.Unlock()
		return sync.OnceFunc(func() {
			localChannelSlots.Lock()
			defer localChannelSlots.Unlock()
			localChannelSlots.counts[channelID]--
			if localChannelSlots.counts[channelID] == 0 {
				delete(localChannelSlots.counts, channelID)
			}
		}), nil
	}
	client := common.RDB
	if client == nil {
		return nil, errors.New("channel concurrency store unavailable")
	}
	key, token := fmt.Sprintf("channel:concurrency:%d", channelID), uuid.NewString()
	callCtx, stop := context.WithTimeout(ctx, 2*time.Second)
	acquired, err := acquireChannelLease.Run(callCtx, client, []string{key}, limit, token, channelLeaseTTL.Milliseconds()).Int()
	stop()
	if err != nil {
		return nil, err
	}
	if acquired == 0 {
		return nil, ErrChannelConcurrencyFull
	}
	done, finished := make(chan struct{}), make(chan struct{})
	go func() {
		defer close(finished)
		ticker := time.NewTicker(channelLeaseTTL / 3)
		defer ticker.Stop()
		for {
			select {
			case <-done:
				return
			case <-ticker.C:
				renewCtx, stop := context.WithTimeout(context.Background(), 2*time.Second)
				renewed, err := renewChannelLease.Run(renewCtx, client, []string{key}, token, channelLeaseTTL.Milliseconds()).Int()
				stop()
				if err != nil || renewed != 1 {
					// 不能确认持有租约时终止上游请求，不能退回本地计数而突破全局上限。
					common.SysError(fmt.Sprintf("channel %d concurrency lease lost: %v", channelID, err))
					cancel()
					return
				}
			}
		}
	}()
	return sync.OnceFunc(func() {
		close(done)
		<-finished
		releaseCtx, stop := context.WithTimeout(context.Background(), 2*time.Second)
		defer stop()
		if err := client.ZRem(releaseCtx, key, token).Err(); err != nil {
			common.SysError(fmt.Sprintf("channel %d concurrency release failed: %v", channelID, err))
		}
	}), nil
}

// ReserveChannelForRequest 只检查已选渠道的容量；满载短暂随机延迟后返回 429，不排队或重新选路。
func ReserveChannelForRequest(c *gin.Context, channel *model.Channel) (*model.Channel, *types.NewAPIError) {
	err := reserveChannelConcurrency(c, channel)
	if err == nil {
		return channel, nil
	}
	if errors.Is(err, ErrChannelConcurrencyFull) {
		// 延迟只缓和拒绝响应，不占并发名额，也不在等待后重新竞争名额。
		timer := time.NewTimer(200*time.Millisecond + time.Duration(rand.Int64N(int64(600*time.Millisecond)+1)))
		defer timer.Stop()
		select {
		case <-timer.C:
		case <-c.Request.Context().Done():
		}
		c.Set(channelConcurrencyRejectedKey, true)
		return nil, types.NewErrorWithStatusCode(ErrChannelConcurrencyFull, "channel_concurrency_limit", http.StatusTooManyRequests, types.ErrOptionWithSkipRetry())
	}
	return nil, types.NewErrorWithStatusCode(errors.New("channel concurrency store unavailable"), "channel_concurrency_unavailable", http.StatusServiceUnavailable, types.ErrOptionWithSkipRetry())
}

// IsChannelConcurrencyRejection 区分本地渠道并发拒绝与上游返回的 429。
func IsChannelConcurrencyRejection(c *gin.Context) bool {
	return c.GetBool(channelConcurrencyRejectedKey)
}
