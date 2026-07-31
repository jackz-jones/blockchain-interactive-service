package middleware

import (
	"context"
	"net/http"
	"sync"
	"time"

	"github.com/jackz-jones/blockchain-interactive-service/internal/store"
	"github.com/zeromicro/go-zero/core/logx"
)

// AnomalyDetector 异常检测器
// 基于滑动窗口统计失败请求频率，超阈值触发告警和自动封禁
type AnomalyDetector struct {
	mu sync.Mutex

	// failureWindows API Key 失败请求窗口: apiKeyID -> *failureWindow
	failureWindows map[uint]*failureWindow

	// bannedKeys 已封禁的 API Key: apiKeyID -> 封禁到期时间
	bannedKeys map[uint]time.Time

	// repo 数据访问层
	repo store.Repository

	// authCache 用于自动封禁时同步失效认证缓存（可选）
	authCache *APIKeyAuthCache

	// 配置
	windowDuration time.Duration // 检测窗口时长
	maxFailures    int           // 窗口内最大失败次数
	banDuration    time.Duration // 封禁时长

	logger logx.Logger

	// 后台清理协程控制
	stopCh chan struct{}
	once   sync.Once
}

// failureWindow 单个 API Key 的失败窗口
type failureWindow struct {
	timestamps []time.Time
	lastSeen   time.Time
}

// AnomalyDetectorConfig 异常检测器配置
type AnomalyDetectorConfig struct {
	WindowDuration time.Duration // 默认 1 分钟
	MaxFailures    int           // 默认 50 次
	BanDuration    time.Duration // 默认 30 分钟
}

// DefaultAnomalyDetectorConfig 默认配置
func DefaultAnomalyDetectorConfig() *AnomalyDetectorConfig {
	return &AnomalyDetectorConfig{
		WindowDuration: 1 * time.Minute,
		MaxFailures:    50,
		BanDuration:    30 * time.Minute,
	}
}

// NewAnomalyDetector 创建异常检测器
func NewAnomalyDetector(repo store.Repository, cfg *AnomalyDetectorConfig, logger logx.Logger) *AnomalyDetector {
	if cfg == nil {
		cfg = DefaultAnomalyDetectorConfig()
	}
	d := &AnomalyDetector{
		failureWindows: make(map[uint]*failureWindow),
		bannedKeys:     make(map[uint]time.Time),
		repo:           repo,
		windowDuration: cfg.WindowDuration,
		maxFailures:    cfg.MaxFailures,
		banDuration:    cfg.BanDuration,
		logger:         logger,
		stopCh:         make(chan struct{}),
	}
	go d.cleanupLoop()
	return d
}

// Stop 停止后台清理协程
func (d *AnomalyDetector) Stop() {
	d.once.Do(func() { close(d.stopCh) })
}

// SetAuthCache 设置认证缓存引用，用于自动封禁时同步失效
// 允许在构造后延迟注入，避免 svc 层循环依赖
func (d *AnomalyDetector) SetAuthCache(cache *APIKeyAuthCache) {
	d.mu.Lock()
	d.authCache = cache
	d.mu.Unlock()
}

// cleanupLoop 后台清理：每 5 分钟移除 10 分钟无活动的失败窗口与过期封禁
func (d *AnomalyDetector) cleanupLoop() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-d.stopCh:
			return
		case <-ticker.C:
			d.gc(10 * time.Minute)
		}
	}
}

func (d *AnomalyDetector) gc(idleThreshold time.Duration) {
	d.mu.Lock()
	defer d.mu.Unlock()

	now := time.Now()
	cutoff := now.Add(-idleThreshold)

	// 清理长期无活动的失败窗口
	for id, w := range d.failureWindows {
		if w.lastSeen.Before(cutoff) {
			delete(d.failureWindows, id)
		}
	}
	// 清理已到期的封禁
	for id, expiry := range d.bannedKeys {
		if now.After(expiry) {
			delete(d.bannedKeys, id)
		}
	}
}

// IsKeyBanned 检查 API Key 是否被封禁
func (d *AnomalyDetector) IsKeyBanned(apiKeyID uint) bool {
	d.mu.Lock()
	defer d.mu.Unlock()

	banExpiry, exists := d.bannedKeys[apiKeyID]
	if !exists {
		return false
	}

	// 检查封禁是否已过期
	if time.Now().After(banExpiry) {
		delete(d.bannedKeys, apiKeyID)
		return false
	}

	return true
}

// RecordFailure 记录一次失败请求
func (d *AnomalyDetector) RecordFailure(apiKeyID uint) {
	d.mu.Lock()
	defer d.mu.Unlock()

	now := time.Now()
	windowStart := now.Add(-d.windowDuration)

	// 获取或创建窗口
	w, exists := d.failureWindows[apiKeyID]
	if !exists {
		w = &failureWindow{timestamps: make([]time.Time, 0, 8)}
		d.failureWindows[apiKeyID] = w
	}

	// 清理过期记录（正确的滑动窗口算法）
	w.timestamps = pruneExpired(w.timestamps, windowStart)

	// 添加本次失败
	w.timestamps = append(w.timestamps, now)
	w.lastSeen = now

	// 检查是否超过阈值
	if len(w.timestamps) >= d.maxFailures {
		d.banKey(apiKeyID)
	}
}

// banKey 封禁 API Key（调用方须持有 d.mu）
func (d *AnomalyDetector) banKey(apiKeyID uint) {
	banExpiry := time.Now().Add(d.banDuration)
	d.bannedKeys[apiKeyID] = banExpiry

	// 清理失败窗口
	delete(d.failureWindows, apiKeyID)

	d.logger.Errorf("[Security] API Key %d auto-banned until %s due to excessive failures",
		apiKeyID, banExpiry.Format(time.RFC3339))

	// 异步更新数据库中的 API Key 状态为 revoked，成功后同步失效认证缓存
	repo := d.repo
	authCache := d.authCache
	go func(id uint) {
		if repo == nil {
			return
		}
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := repo.UpdateAPIKeyStatusByID(ctx, id, "revoked"); err != nil {
			logx.Errorf("[Security] failed to revoke API Key %d in DB: %v", id, err)
			return
		}
		if authCache != nil {
			authCache.InvalidateByKeyID(id)
		}
	}(apiKeyID)
}

// UnbanKey 手动解封 API Key
func (d *AnomalyDetector) UnbanKey(apiKeyID uint) {
	d.mu.Lock()
	defer d.mu.Unlock()

	delete(d.bannedKeys, apiKeyID)
	delete(d.failureWindows, apiKeyID)

	d.logger.Infof("[Security] API Key %d manually unbanned", apiKeyID)
}

// GetBannedKeys 获取所有被封禁的 API Key
func (d *AnomalyDetector) GetBannedKeys() map[uint]time.Time {
	d.mu.Lock()
	defer d.mu.Unlock()

	result := make(map[uint]time.Time)
	now := time.Now()
	for id, expiry := range d.bannedKeys {
		if expiry.After(now) {
			result[id] = expiry
		}
	}
	return result
}

// HTTPAnomalyMiddleware HTTP 异常检测中间件
func HTTPAnomalyMiddleware(detector *AnomalyDetector) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			apiKeyID := GetAPIKeyIDFromHTTP(r)
			if apiKeyID > 0 && detector.IsKeyBanned(apiKeyID) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusForbidden)
				_, _ = w.Write([]byte(
					`{"code":403,"message":"api key temporarily banned",` +
						`"ban_info":"contact support to unban"}`))
				return
			}

			// 包装 ResponseWriter 以检测失败响应
			wrapped := &anomalyResponseWriter{
				ResponseWriter: w,
				statusCode:     http.StatusOK,
			}
			next.ServeHTTP(wrapped, r)

			// 如果响应是错误状态码，记录失败
			if apiKeyID > 0 && wrapped.statusCode >= 400 {
				detector.RecordFailure(apiKeyID)
			}
		})
	}
}

// anomalyResponseWriter 包装 ResponseWriter
type anomalyResponseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (w *anomalyResponseWriter) WriteHeader(code int) {
	w.statusCode = code
	w.ResponseWriter.WriteHeader(code)
}
