package sdk

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"

	"github.com/jackz-jones/blockchain-interactive-service/internal/config"
	"github.com/jackz-jones/blockchain-interactive-service/internal/store"

	commonEvent "github.com/jackz-jones/common/event"
	"github.com/zeromicro/go-zero/core/logx"
)

// TenantSDKManager 租户级 SDK 客户端管理器
// 支持按租户 ID + 链名称获取对应的 SDK 客户端，实现资源隔离
type TenantSDKManager struct {
	// tenantClients 租户级 SDK 客户端缓存: "chain:{chainConfigID}" -> ChainSdkInterface
	tenantClients sync.Map

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
	return &TenantSDKManager{
		factory:     factory,
		repo:        repo,
		redisClient: redisClient,
		logConf:     logConf,
		logger:      logger,
	}
}

// chainCacheKey 生成基于链配置 ID 的缓存 key
func chainCacheKey(chainConfigID uint) string {
	return fmt.Sprintf("chain:%d", chainConfigID)
}

// GetTenantSDKClient 获取租户级 SDK 客户端
// 如果缓存中存在则直接返回，否则从数据库加载配置并创建客户端
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
	if client, ok := m.tenantClients.Load(key); ok {
		return client.(ChainSdkInterface), nil
	}

	// 解析 SDK 配置 JSON
	var sdkConf config.SdkConf
	if chainConfig.SdkConf != "" {
		if err := json.Unmarshal([]byte(chainConfig.SdkConf), &sdkConf); err != nil {
			return nil, fmt.Errorf("parse sdk conf json: %w", err)
		}
	}

	// 加载合约配置
	contractConfigs, err := m.repo.ListContractConfigsByChain(ctx, chainConfig.ID)
	if err != nil {
		return nil, fmt.Errorf("query contract configs: %w", err)
	}
	contractConfs := buildContractConfs(contractConfigs)

	// 创建 SDK 客户端
	client, err := m.createSDKClient(ctx, chainConfig.ChainType, chainName, &sdkConf, contractConfs)
	if err != nil {
		return nil, err
	}

	// 缓存（使用 chainConfigID 作为 key）
	m.tenantClients.Store(key, client)
	m.logger.Infof("created tenant SDK client: tenant=%d, chain=%s, chainConfigID=%d", tenantID, chainName, chainConfig.ID)

	return client, nil
}

// InvalidateTenantCache 使租户链配置缓存失效（配置变更时调用）
func (m *TenantSDKManager) InvalidateTenantCache(tenantID uint, chainName string) {
	// 先查询获取 chainConfigID
	chainConfig, err := m.repo.GetChainConfig(context.Background(), tenantID, chainName)
	if err != nil || chainConfig == nil {
		// 如果查不到，尝试遍历清理
		m.logger.Infof(
			"invalidateTenantCache: cannot find chain config, fallback to range delete: tenant=%d, chain=%s",
			tenantID, chainName)
		m.tenantClients.Range(func(key, value interface{}) bool {
			if client, ok := value.(ChainSdkInterface); ok {
				client.Stop()
			}
			m.tenantClients.Delete(key)
			return true
		})
		return
	}

	key := chainCacheKey(chainConfig.ID)

	// 如果存在旧客户端，先停止
	if old, ok := m.tenantClients.LoadAndDelete(key); ok {
		if client, ok := old.(ChainSdkInterface); ok {
			client.Stop()
		}
	}

	m.logger.Infof("invalidated tenant SDK cache: tenant=%d, chain=%s, chainConfigID=%d",
		tenantID, chainName, chainConfig.ID)
}

// InvalidateTenantCacheByID 使指定 chainConfigID 的缓存失效
func (m *TenantSDKManager) InvalidateTenantCacheByID(chainConfigID uint) {
	key := chainCacheKey(chainConfigID)
	if old, ok := m.tenantClients.LoadAndDelete(key); ok {
		if client, ok := old.(ChainSdkInterface); ok {
			client.Stop()
		}
	}
	m.logger.Infof("invalidated tenant SDK cache by ID: chainConfigID=%d", chainConfigID)
}

// InvalidateAllTenantCache 使某个租户的所有链配置缓存失效
func (m *TenantSDKManager) InvalidateAllTenantCache(tenantID uint) {
	// 查询该租户的所有链配置
	configs, err := m.repo.ListChainConfigsByTenant(context.Background(), tenantID)
	if err != nil {
		m.logger.Errorf("InvalidateAllTenantCache: list chain configs failed: %v", err)
		return
	}
	for _, cfg := range configs {
		key := chainCacheKey(cfg.ID)
		if old, ok := m.tenantClients.LoadAndDelete(key); ok {
			if client, ok := old.(ChainSdkInterface); ok {
				client.Stop()
			}
		}
	}
	m.logger.Infof("invalidated all tenant SDK cache: tenant=%d", tenantID)
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
	m.tenantClients.Range(func(key, value interface{}) bool {
		if client, ok := value.(ChainSdkInterface); ok {
			client.Stop()
		}
		m.tenantClients.Delete(key)
		return true
	})
	m.logger.Info("stopped all tenant SDK clients")
}

// StopSubscription 停止指定合约的订阅
// 通过使缓存失效来停止订阅（Stop 会终止订阅 goroutine），下次重建时不会重新订阅已禁用的合约
func (m *TenantSDKManager) StopSubscription(tenantID uint, chainName string) {
	m.InvalidateTenantCache(tenantID, chainName)
}

// StopAllSubscriptions 停止指定链下所有合约的订阅
// 当链配置被删除时调用
func (m *TenantSDKManager) StopAllSubscriptions(tenantID uint, chainName string) {
	// 清除该链的 SubscribeFlag
	prefix := fmt.Sprintf("%d_%s_", tenantID, chainName)
	SubscribeFlag.Range(func(key, value interface{}) bool {
		if k, ok := key.(string); ok && strings.HasPrefix(k, prefix) {
			SubscribeFlag.Delete(key)
		}
		return true
	})

	// 使缓存失效并停止客户端
	m.InvalidateTenantCache(tenantID, chainName)
}

// RestartSubscription 重启指定链的订阅
// 通过使缓存失效来触发重建，重建时会根据最新配置启动订阅
func (m *TenantSDKManager) RestartSubscription(tenantID uint, chainName string) {
	m.InvalidateTenantCache(tenantID, chainName)
	m.logger.Infof("restart subscription triggered: tenant=%d, chain=%s", tenantID, chainName)
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
	sdkConf *config.SdkConf, contractConfs map[string]*config.ContractConf) (ChainSdkInterface, error) {

	chainConf := &config.ChainConf{
		ChainType:     chainType,
		SdkConf:       *sdkConf,
		ContractConfs: contractConfs,
	}

	client, err := m.factory(ctx, chainName, strings.ToLower(chainType), chainConf, m.logConf, m.redisClient)
	if err != nil {
		return nil, fmt.Errorf("create client for chain '%s' (type=%s): %w", chainName, chainType, err)
	}

	return client, nil
}

// buildContractConfs 将数据库合约配置转换为内存配置格式
func buildContractConfs(dbConfigs []*store.TenantContractConfig) map[string]*config.ContractConf {
	result := make(map[string]*config.ContractConf)
	for _, dbConf := range dbConfigs {
		contractConf := &config.ContractConf{
			ContractName: dbConf.ContractName,
			ContractAddr: dbConf.ContractAddr,
			Abi:          dbConf.AbiJSON,
		}

		// 解析额外配置
		if dbConf.ExtraConf != "" {
			_ = json.Unmarshal([]byte(dbConf.ExtraConf), contractConf)
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
