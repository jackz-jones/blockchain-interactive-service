package middleware

import (
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/zeromicro/go-zero/core/logx"
)

// RateLimiter 基于滑动窗口的限流器
type RateLimiter struct {
	// mu 保护 windows 的并发访问
	mu sync.Mutex

	// windows 租户限流窗口: tenantID -> *slidingWindow
	windows map[uint]*slidingWindow

	// defaultLimit 默认 QPS 限制
	defaultLimit int
}

// slidingWindow 滑动窗口
type slidingWindow struct {
	requests []time.Time
	limit    int
}

// NewRateLimiter 创建限流器
func NewRateLimiter(defaultLimit int) *RateLimiter {
	if defaultLimit <= 0 {
		defaultLimit = 10
	}
	return &RateLimiter{
		windows:      make(map[uint]*slidingWindow),
		defaultLimit: defaultLimit,
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

	// 清理过期请求
	validIdx := 0
	for i, t := range window.requests {
		if t.After(windowStart) {
			validIdx = i
			break
		}
		if i == len(window.requests)-1 {
			validIdx = len(window.requests)
		}
	}
	window.requests = window.requests[validIdx:]

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
