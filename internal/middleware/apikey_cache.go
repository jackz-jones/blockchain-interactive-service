package middleware

import (
	"context"
	"crypto/subtle"
	"sync"
	"time"

	"github.com/jackz-jones/blockchain-interactive-service/internal/store"
)

// apiKeyAuthInfo 缓存中记录的 API Key 认证要素
// 只保留认证/授权路径所需最小字段，避免持有超大对象
type apiKeyAuthInfo struct {
	APIKeyID    uint
	TenantID    uint
	UserID      uint
	UserRole    store.UserRole
	Status      string
	ExpiresAt   *time.Time
	IPWhitelist string
	// TenantStatus 缓存租户状态，避免每次请求都查 tenant 表
	TenantStatus store.TenantStatus
	// keyDigest 用于常量时间比较，避免作为 map key 的字符串比较可能带来的时序副作用
	keyDigest [32]byte
}

// apiKeyCacheEntry 缓存条目 + 过期时间
type apiKeyCacheEntry struct {
	info      *apiKeyAuthInfo
	expiresAt time.Time
}

// APIKeyAuthCache API Key 认证缓存
//
// 目的：
//   - 消除认证阶段对 api_key / tenant / user 三张表的高频重复查询
//   - 支持按 TTL 自动失效；支持按 keyID 主动失效（撤销/更新时）
//
// 线程安全：sync.RWMutex 保护 map；命中路径完全走 RLock。
type APIKeyAuthCache struct {
	mu      sync.RWMutex
	byKey   map[string]*apiKeyCacheEntry // rawKey -> entry
	byKeyID map[uint]string              // keyID  -> rawKey (用于按 ID 精确失效)

	// 认证结果 TTL；命中该 TTL 内不再查 DB
	ttl time.Duration

	// 负缓存 TTL（可选）：查不到的 key 短期内不再查 DB，防止暴力扫描打穿 DB
	negTTL      time.Duration
	notFoundSet map[string]time.Time // rawKey -> negativeExpiresAt

	// last_used 节流：同一 keyID 在 lastUsedThrottle 内只写一次 DB
	lastUsedThrottle time.Duration
	lastUsedAt       map[uint]time.Time
	lastUsedMu       sync.Mutex

	stopCh   chan struct{}
	stopOnce sync.Once
}

// APIKeyAuthCacheConfig 缓存配置
type APIKeyAuthCacheConfig struct {
	TTL              time.Duration
	NegativeTTL      time.Duration
	LastUsedThrottle time.Duration
	CleanupInterval  time.Duration
}

// DefaultAPIKeyCacheConfig 默认缓存参数
func DefaultAPIKeyCacheConfig() APIKeyAuthCacheConfig {
	return APIKeyAuthCacheConfig{
		TTL:              60 * time.Second,
		NegativeTTL:      5 * time.Second,
		LastUsedThrottle: 60 * time.Second,
		CleanupInterval:  2 * time.Minute,
	}
}

// NewAPIKeyAuthCache 创建认证缓存并启动后台清理协程
func NewAPIKeyAuthCache(cfg APIKeyAuthCacheConfig) *APIKeyAuthCache {
	if cfg.TTL <= 0 {
		cfg.TTL = 60 * time.Second
	}
	if cfg.LastUsedThrottle <= 0 {
		cfg.LastUsedThrottle = 60 * time.Second
	}
	if cfg.CleanupInterval <= 0 {
		cfg.CleanupInterval = 2 * time.Minute
	}
	c := &APIKeyAuthCache{
		byKey:            make(map[string]*apiKeyCacheEntry),
		byKeyID:          make(map[uint]string),
		notFoundSet:      make(map[string]time.Time),
		ttl:              cfg.TTL,
		negTTL:           cfg.NegativeTTL,
		lastUsedThrottle: cfg.LastUsedThrottle,
		lastUsedAt:       make(map[uint]time.Time),
		stopCh:           make(chan struct{}),
	}
	go c.cleanupLoop(cfg.CleanupInterval)
	return c
}

// Stop 停止后台清理协程
func (c *APIKeyAuthCache) Stop() {
	c.stopOnce.Do(func() { close(c.stopCh) })
}

// Lookup 从缓存中查找 API Key 认证信息
// 命中返回 (info, true)；未命中或已过期返回 (nil, false)
// 命中路径使用常量时间比较，避免通过响应时长推断 key 内容
func (c *APIKeyAuthCache) Lookup(rawKey string) (*apiKeyAuthInfo, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	entry, ok := c.byKey[rawKey]
	if !ok || entry == nil {
		return nil, false
	}
	if time.Now().After(entry.expiresAt) {
		return nil, false
	}
	// 使用常量时间比较（虽已通过 map lookup 定位到条目，这里以 digest 二次比较作为纵深防御）
	sum := hashRawKey(rawKey)
	if subtle.ConstantTimeCompare(sum[:], entry.info.keyDigest[:]) != 1 {
		return nil, false
	}
	return entry.info, true
}

// LookupNotFound 判断 rawKey 是否在负缓存中（近期查过 DB 且不存在）
func (c *APIKeyAuthCache) LookupNotFound(rawKey string) bool {
	if c.negTTL <= 0 {
		return false
	}
	c.mu.RLock()
	defer c.mu.RUnlock()
	exp, ok := c.notFoundSet[rawKey]
	if !ok {
		return false
	}
	return time.Now().Before(exp)
}

// Store 写入缓存
func (c *APIKeyAuthCache) Store(rawKey string, info *apiKeyAuthInfo) {
	if info == nil {
		return
	}
	info.keyDigest = hashRawKey(rawKey)
	entry := &apiKeyCacheEntry{
		info:      info,
		expiresAt: time.Now().Add(c.ttl),
	}
	c.mu.Lock()
	c.byKey[rawKey] = entry
	c.byKeyID[info.APIKeyID] = rawKey
	// 若之前在负缓存中，一并清除
	delete(c.notFoundSet, rawKey)
	c.mu.Unlock()
}

// StoreNotFound 写入负缓存
func (c *APIKeyAuthCache) StoreNotFound(rawKey string) {
	if c.negTTL <= 0 {
		return
	}
	c.mu.Lock()
	c.notFoundSet[rawKey] = time.Now().Add(c.negTTL)
	c.mu.Unlock()
}

// InvalidateByKeyID 按 API Key ID 精确失效缓存（撤销/更新场景）
func (c *APIKeyAuthCache) InvalidateByKeyID(keyID uint) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if raw, ok := c.byKeyID[keyID]; ok {
		delete(c.byKey, raw)
		delete(c.byKeyID, keyID)
	}
}

// InvalidateAll 清空所有缓存
func (c *APIKeyAuthCache) InvalidateAll() {
	c.mu.Lock()
	c.byKey = make(map[string]*apiKeyCacheEntry)
	c.byKeyID = make(map[uint]string)
	c.notFoundSet = make(map[string]time.Time)
	c.mu.Unlock()
}

// ShouldUpdateLastUsed 判断是否应该在本次请求中更新 last_used
// 若上次更新距今超过 lastUsedThrottle，则允许并把 now 记为本次时间
func (c *APIKeyAuthCache) ShouldUpdateLastUsed(keyID uint) bool {
	if c.lastUsedThrottle <= 0 {
		return true
	}
	now := time.Now()
	c.lastUsedMu.Lock()
	defer c.lastUsedMu.Unlock()
	last, ok := c.lastUsedAt[keyID]
	if !ok || now.Sub(last) >= c.lastUsedThrottle {
		c.lastUsedAt[keyID] = now
		return true
	}
	return false
}

// UpdateLastUsedAsync 在允许的节流窗口内，异步写 last_used 到 DB
// 已被 ShouldUpdateLastUsed 门禁保护，此方法不会重复写。
func (c *APIKeyAuthCache) UpdateLastUsedAsync(repo store.Repository, keyID uint) {
	if !c.ShouldUpdateLastUsed(keyID) {
		return
	}
	go func() {
		_ = repo.UpdateAPIKeyLastUsed(context.Background(), keyID, time.Now())
	}()
}

// cleanupLoop 后台周期扫描，删除过期条目
func (c *APIKeyAuthCache) cleanupLoop(interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-c.stopCh:
			return
		case <-ticker.C:
			c.cleanupOnce(time.Now())
		}
	}
}

func (c *APIKeyAuthCache) cleanupOnce(now time.Time) {
	c.mu.Lock()
	defer c.mu.Unlock()
	for k, entry := range c.byKey {
		if now.After(entry.expiresAt) {
			delete(c.byKey, k)
			if entry.info != nil {
				delete(c.byKeyID, entry.info.APIKeyID)
			}
		}
	}
	for k, exp := range c.notFoundSet {
		if now.After(exp) {
			delete(c.notFoundSet, k)
		}
	}
}
