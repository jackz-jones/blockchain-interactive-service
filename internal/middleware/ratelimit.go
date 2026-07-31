package middleware

import (
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/zeromicro/go-zero/core/logx"
)

// pruneExpired 移除切片中早于 windowStart 的时间戳；
// 采用"找到第一个 t.After(windowStart) 的索引"，返回 requests[idx:]。
// 若切片全部过期，返回空切片；若无过期项，原样返回。
func pruneExpired(requests []time.Time, windowStart time.Time) []time.Time {
	if len(requests) == 0 {
		return requests
	}
	// 找到第一个在窗口内的索引
	idx := len(requests) // 默认全部过期
	for i, t := range requests {
		if t.After(windowStart) {
			idx = i
			break
		}
	}
	if idx == 0 {
		return requests
	}
	if idx >= len(requests) {
		return requests[:0]
	}
	return requests[idx:]
}

// RateLimiter 基于滑动窗口的限流器
type RateLimiter struct {
	// mu 保护 windows 的并发访问
	mu sync.Mutex

	// windows 租户限流窗口: tenantID -> *slidingWindow
	windows map[uint]*slidingWindow

	// defaultLimit 默认 QPS 限制
	defaultLimit int

	// 后台清理协程控制
	stopCh chan struct{}
	once   sync.Once
}

// slidingWindow 滑动窗口
type slidingWindow struct {
	requests []time.Time
	limit    int
	lastSeen time.Time // 最近一次访问时间，用于后台淘汰
}

// NewRateLimiter 创建限流器
func NewRateLimiter(defaultLimit int) *RateLimiter {
	if defaultLimit <= 0 {
		defaultLimit = 10
	}
	rl := &RateLimiter{
		windows:      make(map[uint]*slidingWindow),
		defaultLimit: defaultLimit,
		stopCh:       make(chan struct{}),
	}
	go rl.cleanupLoop()
	return rl
}

// Stop 停止后台清理协程
func (rl *RateLimiter) Stop() {
	rl.once.Do(func() { close(rl.stopCh) })
}

// cleanupLoop 后台清理协程：每 5 分钟移除 10 分钟无活动的租户窗口
func (rl *RateLimiter) cleanupLoop() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-rl.stopCh:
			return
		case <-ticker.C:
			rl.gc(10 * time.Minute)
		}
	}
}

// gc 移除超过 idleThreshold 未活跃的窗口
func (rl *RateLimiter) gc(idleThreshold time.Duration) {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	cutoff := time.Now().Add(-idleThreshold)
	for tid, w := range rl.windows {
		if w.lastSeen.Before(cutoff) {
			delete(rl.windows, tid)
		}
	}
}

// Allow 检查是否允许请求通过
func (rl *RateLimiter) Allow(tenantID uint, limit int) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	if limit <= 0 {
		limit = rl.defaultLimit
	}

	window, exists := rl.windows[tenantID]
	if !exists {
		window = &slidingWindow{
			requests: make([]time.Time, 0, limit),
			limit:    limit,
		}
		rl.windows[tenantID] = window
	}

	now := time.Now()
	windowStart := now.Add(-time.Second) // 1 秒滑动窗口

	// 清理过期请求（正确的滑动窗口算法）
	window.requests = pruneExpired(window.requests, windowStart)
	window.lastSeen = now

	// 检查是否超限
	if len(window.requests) >= limit {
		return false
	}

	// 记录本次请求
	window.requests = append(window.requests, now)
	return true
}

// HTTPRateLimitMiddleware HTTP 限流中间件
func HTTPRateLimitMiddleware(limiter *RateLimiter) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			tenantID := GetTenantIDFromHTTP(r)
			if tenantID == 0 {
				// 未认证的请求不做限流（认证中间件会拦截）
				next.ServeHTTP(w, r)
				return
			}

			if !limiter.Allow(tenantID, 0) {
				logx.WithContext(r.Context()).Infof("rate limit exceeded for tenant %d", tenantID)
				w.Header().Set("Content-Type", "application/json")
				w.Header().Set("Retry-After", "1")
				w.WriteHeader(http.StatusTooManyRequests)
				_, _ = w.Write([]byte(fmt.Sprintf(
					`{"code":429,"message":"rate limit exceeded, please retry after 1 second","tenant_id":%d}`,
					tenantID)))
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
