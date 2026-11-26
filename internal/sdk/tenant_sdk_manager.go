package sdk

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/jackz-jones/blockchain-interactive-service/internal/store"

	commonEvent "github.com/jackz-jones/common/event"
	"github.com/zeromicro/go-zero/core/logx"
	"golang.org/x/sync/singleflight"
)

// tenantClientEntry 是 tenantClients 缓存中的条目
// 除持有 SDK 客户端外，还记录最近访问时间用于 TTL 淘汰
type tenantClientEntry struct {
	client       ChainSdkInterface
	lastAccessNs int64 // atomic: unix nano 时间戳
}

func (e *tenantClientEntry) touch() {
	atomic.StoreInt64(&e.lastAccessNs, time.Now().UnixNano())
}

func (e *tenantClientEntry) lastAccess() time.Time {
	return time.Unix(0, atomic.LoadInt64(&e.lastAccessNs))
}

// 默认 TTL / 清理间隔
const (
	defaultTenantSDKIdleTTL      = 30 * time.Minute
	defaultTenantSDKCleanupEvery = 5 * time.Minute
)

// BuildSDKConf 根据链配置构建 SDKConf
func BuildSDKConf(chainConfig *store.TenantChainConfig, nodes []*store.TenantChainNode) SDKConf {
	var sdkConf SDKConf

	switch strings.ToLower(chainConfig.ChainType) {
	case "chainmaker":
		cmConf := ChainMakerConf{
			ChainId:     chainConfig.ChainId,
			AuthType:    chainConfig.AuthType,
			OrgId:       chainConfig.OrgId,
			HashType:    chainConfig.HashType,
			SignKey:     chainConfig.SignKey,
			SignCert:    chainConfig.SignCert,
			UserTlsKey:  chainConfig.UserTlsKey,
			UserTlsCert: chainConfig.UserTlsCert,
			UserEncKey:  chainConfig.UserEncKey,
			UserEncCert: chainConfig.UserEncCert,
			ProxyUrl:    chainConfig.ProxyUrl,
		}
		// 构建节点配置
		for _, n := range nodes {
			cmConf.Nodes = append(cmConf.Nodes, ChainMakerNodeConf{
				NodeAddr:    n.NodeAddr,
				ConnCnt:     n.ConnCnt,
				EnableTls:   n.EnableTls,
				TlsHostName: n.TlsHostName,
				CaCert:      n.CaCert,
			})
		}
		sdkConf.ChainMakerConf = cmConf

	case "ethereum":
		sdkConf.EthConf = EthConf{
			ChainId:      chainConfig.EthChainId,
			HttpUrl:      chainConfig.HttpUrl,
			WebsocketUrl: chainConfig.WebsocketUrl,
			PrivateKey:   chainConfig.PrivateKey,
		}

	case "solana":
		sdkConf.SolanaConf = SolanaConf{
			RpcUrl:          chainConfig.SolRpcUrl,
			PrivateKey:      chainConfig.SolPrivateKey,
			CommitmentLevel: chainConfig.CommitmentLevel,
			SkipPreflight:   chainConfig.SkipPreflight,
			MaxRetries:      chainConfig.MaxRetries,
		}
	}

	return sdkConf
}

// TenantSDKManager 租户级 SDK 客户端管理器
// 支持按租户 ID + 链名称获取对应的 SDK 客户端，实现资源隔离
type TenantSDKManager struct {
	// tenantClients 租户级 SDK 客户端缓存: "chain:{chainConfigID}" -> *tenantClientEntry
	tenantClients sync.Map

	// tenantIndex tenantID -> map[chainConfigID]struct{}，用于按租户维度精确失效缓存，避免"fallback 全清"
	tenantIndex   map[uint]map[uint]struct{}
	tenantIndexMu sync.Mutex

	// sfGroup 用于抑制并发首次创建同一 chainConfigID 客户端时的重复初始化
	sfGroup singleflight.Group

	// idleTTL 空闲客户端在无访问后被淘汰的时长
	idleTTL time.Duration

	// stopCh 用于停止后台清理 goroutine
	stopCh   chan struct{}
	stopOnce sync.Once

	// factory 链客户端工厂函数，通过它创建链客户端
	factory ChainClientFactory

	// repo 数据访问层，用于查询租户链配置
	repo store.Repository

	// redisClient Redis 客户端
	redisClient *commonEvent.RedisClient

	// logConf 日志配置
	logConf logx.LogConf

	// logger 日志
	logger logx.Logger
}

// NewTenantSDKManager 创建租户级 SDK 管理器
func NewTenantSDKManager(repo store.Repository, factory ChainClientFactory, redisClient *commonEvent.RedisClient,
	logConf logx.LogConf, logger logx.Logger) *TenantSDKManager {
	m := &TenantSDKManager{
		factory:     factory,
		repo:        repo,
		redisClient: redisClient,
		logConf:     logConf,
		logger:      logger,
		tenantIndex: make(map[uint]map[uint]struct{}),
		idleTTL:     defaultTenantSDKIdleTTL,
		stopCh:      make(chan struct{}),
	}
	// 启动后台清理协程
	go m.cleanupLoop(defaultTenantSDKCleanupEvery)
	return m
}

// chainCacheKey 生成基于链配置 ID 的缓存 key
func chainCacheKey(chainConfigID uint) string {
	return fmt.Sprintf("chain:%d", chainConfigID)
}

// indexAdd 将 chainConfigID 加入指定 tenantID 的索引
func (m *TenantSDKManager) indexAdd(tenantID, chainConfigID uint) {
	m.tenantIndexMu.Lock()
	defer m.tenantIndexMu.Unlock()
	set, ok := m.tenantIndex[tenantID]
	if !ok {
		set = make(map[uint]struct{})
		m.tenantIndex[tenantID] = set
	}
	set[chainConfigID] = struct{}{}
}

// indexRemove 从索引中移除 chainConfigID
func (m *TenantSDKManager) indexRemove(chainConfigID uint) {
	m.tenantIndexMu.Lock()
	defer m.tenantIndexMu.Unlock()
	for tid, set := range m.tenantIndex {
		if _, ok := set[chainConfigID]; ok {
			delete(set, chainConfigID)
			if len(set) == 0 {
				delete(m.tenantIndex, tid)
			}
			return
		}
	}
}

// indexListByTenant 返回指定租户下所有已缓存的 chainConfigID
func (m *TenantSDKManager) indexListByTenant(tenantID uint) []uint {
	m.tenantIndexMu.Lock()
	defer m.tenantIndexMu.Unlock()
	set, ok := m.tenantIndex[tenantID]
	if !ok {
		return nil
	}
	ids := make([]uint, 0, len(set))
	for id := range set {
		ids = append(ids, id)
	}
	return ids
}

// evictByKey 从缓存中删除并停止对应客户端；同步更新索引
func (m *TenantSDKManager) evictByKey(key string, chainConfigID uint) {
	if old, ok := m.tenantClients.LoadAndDelete(key); ok {
		if entry, ok := old.(*tenantClientEntry); ok && entry.client != nil {
			_ = entry.client.Stop()
		}
	}
	if chainConfigID != 0 {
		m.indexRemove(chainConfigID)
	}
}

// cleanupLoop 后台清理协程，周期性淘汰空闲超过 idleTTL 的客户端
func (m *TenantSDKManager) cleanupLoop(interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-m.stopCh:
			return
		case <-ticker.C:
			m.cleanupIdle()
		}
	}
}

// cleanupIdle 扫描一遍缓存，淘汰空闲超过 idleTTL 的条目
func (m *TenantSDKManager) cleanupIdle() {
	now := time.Now()
	m.tenantClients.Range(func(key, value interface{}) bool {
		entry, ok := value.(*tenantClientEntry)
		if !ok {
			return true
		}
		if now.Sub(entry.lastAccess()) < m.idleTTL {
			return true
		}
		k, _ := key.(string)
		// 从 key "chain:<id>" 提取 id 用于索引清理
		var cid uint
		_, _ = fmt.Sscanf(k, "chain:%d", &cid)
		m.evictByKey(k, cid)
		m.logger.Infof("tenant SDK client evicted due to idle timeout: key=%s, idle=%s", k, now.Sub(entry.lastAccess()))
		return true
	})
}

// GetTenantSDKClient 获取租户级 SDK 客户端
// 如果缓存中存在则直接返回，否则从数据库加载配置并创建客户端。
// 并发首次创建同一 chainConfigID 时会通过 singleflight 合并，避免重复创建与资源泄漏。
func (m *TenantSDKManager) GetTenantSDKClient(
	ctx context.Context, tenantID uint, chainName string,
) (ChainSdkInterface, error) {
	// 从数据库加载租户链配置
	chainConfig, err := m.repo.GetChainConfig(ctx, tenantID, chainName)
	if err != nil {
		return nil, fmt.Errorf("query tenant chain config: %w", err)
	}
	if chainConfig == nil {
		return nil, fmt.Errorf("chain config '%s' not found for tenant %d", chainName, tenantID)
	}
	if !chainConfig.Enable {
		return nil, fmt.Errorf("chain '%s' is disabled for tenant %d", chainName, tenantID)
	}

	// 使用 chainConfigID 作为缓存 key
	key := chainCacheKey(chainConfig.ID)

	// 尝试从缓存获取
	if v, ok := m.tenantClients.Load(key); ok {
		if entry, ok := v.(*tenantClientEntry); ok && entry.client != nil {
			entry.touch()
			return entry.client, nil
		}
	}

	// 并发保护：同一 key 的首次创建仅执行一次
	v, err, _ := m.sfGroup.Do(key, func() (interface{}, error) {
		// double-check
		if cached, ok := m.tenantClients.Load(key); ok {
			if entry, ok := cached.(*tenantClientEntry); ok && entry.client != nil {
				entry.touch()
				return entry.client, nil
			}
		}

		// 查询节点配置
		nodes, err := m.repo.ListChainNodesByConfigID(ctx, chainConfig.ID)
		if err != nil {
			return nil, fmt.Errorf("query chain nodes: %w", err)
		}

		// 从数据库结构化字段构建 SDKConf
		sdkConf := BuildSDKConf(chainConfig, nodes)

		// 加载合约配置
		contractConfigs, err := m.repo.ListContractConfigsByChain(ctx, chainConfig.ID)
		if err != nil {
			return nil, fmt.Errorf("query contract configs: %w", err)
		}
		contractConfs := m.buildContractConfs(contractConfigs)

		// 创建 SDK 客户端
		client, err := m.createSDKClient(ctx, chainConfig.ChainType, chainName, &sdkConf, contractConfs)
		if err != nil {
			return nil, err
		}

		entry := &tenantClientEntry{client: client}
		entry.touch()
		m.tenantClients.Store(key, entry)
		m.indexAdd(tenantID, chainConfig.ID)
		m.logger.Infof("created tenant SDK client: tenant=%d, chain=%s, chainConfigID=%d", tenantID, chainName, chainConfig.ID)
		return client, nil
	})
	if err != nil {
		return nil, err
	}
	if client, ok := v.(ChainSdkInterface); ok {
		return client, nil
	}
	return nil, fmt.Errorf("unexpected cache entry type for key %s", key)
}

// InvalidateTenantCache 使租户链配置缓存失效（配置变更时调用）
// 与旧实现不同：查不到 chainConfig 时，仅清理该租户在索引中记录的 key，
// 而不再遍历删除所有租户的缓存，避免误伤其他租户。
func (m *TenantSDKManager) InvalidateTenantCache(tenantID uint, chainName string) {
	chainConfig, err := m.repo.GetChainConfig(context.Background(), tenantID, chainName)
	if err != nil || chainConfig == nil {
		m.logger.Infof(
			"invalidateTenantCache: cannot find chain config, fallback to tenant-scoped invalidate: tenant=%d, chain=%s",
			tenantID, chainName)
		// 只清理该租户在索引中记录的 chain 缓存，绝不影响其他租户
		for _, cid := range m.indexListByTenant(tenantID) {
			m.evictByKey(chainCacheKey(cid), cid)
		}
		return
	}

	key := chainCacheKey(chainConfig.ID)
	m.evictByKey(key, chainConfig.ID)
	m.logger.Infof("invalidated tenant SDK cache: tenant=%d, chain=%s, chainConfigID=%d",
		tenantID, chainName, chainConfig.ID)
}

// InvalidateTenantCacheByID 使指定 chainConfigID 的缓存失效
func (m *TenantSDKManager) InvalidateTenantCacheByID(chainConfigID uint) {
	m.evictByKey(chainCacheKey(chainConfigID), chainConfigID)
	m.logger.Infof("invalidated tenant SDK cache by ID: chainConfigID=%d", chainConfigID)
}

// InvalidateAllTenantCache 使某个租户的所有链配置缓存失效
func (m *TenantSDKManager) InvalidateAllTenantCache(tenantID uint) {
	// 优先使用内存索引（可覆盖已缓存但 DB 已删除的场景）
	cachedIDs := m.indexListByTenant(tenantID)
	for _, cid := range cachedIDs {
		m.evictByKey(chainCacheKey(cid), cid)
	}
	// 兜底：DB 中记录的所有链配置也一并清理
	configs, err := m.repo.ListChainConfigsByTenant(context.Background(), tenantID)
	if err != nil {
		m.logger.Errorf("InvalidateAllTenantCache: list chain configs failed: %v", err)
		return
	}
	for _, cfg := range configs {
		m.evictByKey(chainCacheKey(cfg.ID), cfg.ID)
	}
	m.logger.Infof("invalidated all tenant SDK cache: tenant=%d, evicted=%d", tenantID, len(cachedIDs)+len(configs))
}

// ListTenantChains 列出租户可用的链名称
func (m *TenantSDKManager) ListTenantChains(ctx context.Context, tenantID uint) ([]string, error) {
	configs, err := m.repo.ListChainConfigsByTenant(ctx, tenantID)
	if err != nil {
		return nil, err
	}

	var chains []string
	for _, c := range configs {
		if c.Enable {
			chains = append(chains, c.ChainName)
		}
	}
	return chains, nil
}

// StopAll 停止所有租户 SDK 客户端
func (m *TenantSDKManager) StopAll() {
	// 停止后台清理协程
	m.stopOnce.Do(func() { close(m.stopCh) })

	m.tenantClients.Range(func(key, value interface{}) bool {
		if entry, ok := value.(*tenantClientEntry); ok && entry.client != nil {
			_ = entry.client.Stop()
		}
		m.tenantClients.Delete(key)
		return true
	})
	m.tenantIndexMu.Lock()
	m.tenantIndex = make(map[uint]map[uint]struct{})
	m.tenantIndexMu.Unlock()
	m.logger.Info("stopped all tenant SDK clients")
}

// StopSubscription 停止指定合约的订阅
// 通过使缓存失效来停止订阅（Stop 会终止订阅 goroutine），下次重建时不会重新订阅已禁用的合约
func (m *TenantSDKManager) StopSubscription(tenantID uint, chainName string) {
	m.InvalidateTenantCache(tenantID, chainName)
}

// StopContractSubscription 停止指定合约的订阅（精确到合约级别）
// 当合约配置被删除时调用，清理对应的 SubscribeFlag 并停止 SDK 客户端
func (m *TenantSDKManager) StopContractSubscription(chainConfigID, contractConfigID uint, tenantID uint, chainName string) {
	// 清理该合约的 SubscribeFlag
	flagKey := SubscribeKeyByID(chainConfigID, contractConfigID)
	SubscribeFlag.Delete(flagKey)
	m.logger.Infof("stopped contract subscription: flagKey=%s, tenant=%d, chain=%s", flagKey, tenantID, chainName)

	// 使缓存失效并停止客户端（会触发订阅 goroutine 退出）
	m.InvalidateTenantCache(tenantID, chainName)
}

// StopAllSubscriptions 停止指定链下所有合约的订阅
// 当链配置被删除时调用
func (m *TenantSDKManager) StopAllSubscriptions(tenantID uint, chainName string, chainConfigID uint) {
	// 清除该链下所有合约的 SubscribeFlag（key 格式为 "db:chainConfigID-contractConfigID"）
	prefix := fmt.Sprintf("db:%d-", chainConfigID)
	SubscribeFlag.Range(func(key, value interface{}) bool {
		if k, ok := key.(string); ok && strings.HasPrefix(k, prefix) {
			SubscribeFlag.Delete(key)
			m.logger.Infof("cleared SubscribeFlag: %s", k)
		}
		return true
	})

	// 使缓存失效并停止客户端
	m.InvalidateTenantCacheByID(chainConfigID)
	m.logger.Infof("stopped all subscriptions for chain: tenant=%d, chain=%s, chainConfigID=%d",
		tenantID, chainName, chainConfigID)
}

// RestartSubscription 重启指定链的订阅
// 通过使缓存失效来触发重建，重建时会根据最新配置启动订阅
func (m *TenantSDKManager) RestartSubscription(tenantID uint, chainName string) {
	m.InvalidateTenantCache(tenantID, chainName)
	m.logger.Infof("restart subscription triggered: tenant=%d, chain=%s", tenantID, chainName)
}

// TestChainConnect 测试链配置的连接可用性
// 通过临时创建 SDK 客户端来验证配置是否可用，验证后立即释放资源
// 返回 nil 表示连接成功，否则返回错误信息
func (m *TenantSDKManager) TestChainConnect(ctx context.Context, chainConfig *store.TenantChainConfig, nodes []*store.TenantChainNode) error {
	sdkConf := BuildSDKConf(chainConfig, nodes)
	client, err := m.createSDKClient(ctx, chainConfig.ChainType, chainConfig.ChainName, &sdkConf, nil)
	if err != nil {
		return fmt.Errorf("connection test failed: %w", err)
	}
	// 验证连接是否真正可用（如获取链 ID / 区块高度等轻量查询）
	// 使用 5 秒超时避免连接验证长时间阻塞
	verifyCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := client.VerifyConnection(verifyCtx); err != nil {
		_ = client.Stop()
		return fmt.Errorf("connection verification failed: %w", err)
	}
	// 立即释放测试客户端资源
	_ = client.Stop()
	m.logger.Infof("chain connection test passed: chain=%s, type=%s", chainConfig.ChainName, chainConfig.ChainType)
	return nil
}

// InvalidateChainSubscriptions 中断指定链下所有合约的订阅协程
// 清除该链下所有合约的 SubscribeFlag，并使 SDK 缓存失效
// 调度器将在下一个轮询周期（3s 内）基于新配置自动重启订阅
func (m *TenantSDKManager) InvalidateChainSubscriptions(chainConfigID uint) {
	// 清除该链下所有合约的 SubscribeFlag（key 格式为 "db:chainConfigID-contractConfigID"）
	prefix := fmt.Sprintf("db:%d-", chainConfigID)
	SubscribeFlag.Range(func(key, value interface{}) bool {
		if k, ok := key.(string); ok && strings.HasPrefix(k, prefix) {
			SubscribeFlag.Delete(key)
			m.logger.Infof("cleared SubscribeFlag for chain update: %s", k)
		}
		return true
	})
	// 使 SDK 缓存失效，停止旧客户端（会触发订阅 goroutine 退出）
	m.InvalidateTenantCacheByID(chainConfigID)
	m.logger.Infof("invalidated chain subscriptions for update: chainConfigID=%d", chainConfigID)
}

// GetClientStatus 获取租户链客户端的运行状态
func (m *TenantSDKManager) GetClientStatus(tenantID uint, chainName string) map[string]interface{} {
	status := map[string]interface{}{
		"tenant_id":     tenantID,
		"chain_name":    chainName,
		"client_active": false,
	}

	chainConfig, err := m.repo.GetChainConfig(context.Background(), tenantID, chainName)
	if err != nil || chainConfig == nil {
		return status
	}

	key := chainCacheKey(chainConfig.ID)
	if _, ok := m.tenantClients.Load(key); ok {
		status["client_active"] = true
		status["chain_config_id"] = chainConfig.ID
	}

	return status
}

// createSDKClient 根据链类型通过工厂函数创建 SDK 客户端
func (m *TenantSDKManager) createSDKClient(ctx context.Context, chainType, chainName string,
	sdkConf *SDKConf, contractConfs map[string]*ContractConf) (ChainSdkInterface, error) {

	chainConf := &ChainConf{
		ChainType:     chainType,
		SDKConf:       *sdkConf,
		ContractConfs: contractConfs,
	}

	client, err := m.factory(ctx, chainName, strings.ToLower(chainType), chainConf, m.logConf, m.redisClient)
	if err != nil {
		return nil, fmt.Errorf("create client for chain '%s' (type=%s): %w", chainName, chainType, err)
	}

	return client, nil
}

// buildContractConfs 将数据库合约配置转换为内存配置格式
// 不再静默忽略 ExtraConf 解析错误，改为记录 Warn 日志便于排查
func (m *TenantSDKManager) buildContractConfs(dbConfigs []*store.TenantContractConfig) map[string]*ContractConf {
	result := make(map[string]*ContractConf)
	for _, dbConf := range dbConfigs {
		contractConf := &ContractConf{
			ContractName: dbConf.ContractName,
			ContractAddr: dbConf.ContractAddr,
			Abi:          dbConf.AbiJSON,
		}

		// 解析额外配置
		if dbConf.ExtraConf != "" {
			if err := json.Unmarshal([]byte(dbConf.ExtraConf), contractConf); err != nil {
				m.logger.Errorf("unmarshal ExtraConf failed: contract=%s, err=%v", dbConf.ContractName, err)
			}
			// 确保核心字段不被覆盖
			contractConf.ContractName = dbConf.ContractName
			contractConf.ContractAddr = dbConf.ContractAddr
			if dbConf.AbiJSON != "" {
				contractConf.Abi = dbConf.AbiJSON
			}
		}

		result[dbConf.ContractName] = contractConf
	}
	return result
}

// buildContractConfs 保留为包级别函数以兼容外部调用点（若有）
// Deprecated: 请使用 (*TenantSDKManager).buildContractConfs
func buildContractConfs(dbConfigs []*store.TenantContractConfig) map[string]*ContractConf {
	result := make(map[string]*ContractConf)
	for _, dbConf := range dbConfigs {
		contractConf := &ContractConf{
			ContractName: dbConf.ContractName,
			ContractAddr: dbConf.ContractAddr,
			Abi:          dbConf.AbiJSON,
		}
		if dbConf.ExtraConf != "" {
			if err := json.Unmarshal([]byte(dbConf.ExtraConf), contractConf); err != nil {
				logx.Errorf("unmarshal ExtraConf failed: contract=%s, err=%v", dbConf.ContractName, err)
			}
			contractConf.ContractName = dbConf.ContractName
			contractConf.ContractAddr = dbConf.ContractAddr
			if dbConf.AbiJSON != "" {
				contractConf.Abi = dbConf.AbiJSON
			}
		}
		result[dbConf.ContractName] = contractConf
	}
	return result
}
