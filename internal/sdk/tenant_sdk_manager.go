package sdk

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"

	"github.com/jackz-jones/blockchain-interactive-service/internal/config"
	"github.com/jackz-jones/blockchain-interactive-service/internal/store"
	pb "github.com/jackz-jones/blockchain-interactive-service/pb"

	commonEvent "github.com/jackz-jones/common/event"
	"github.com/zeromicro/go-zero/core/logx"
)

// TenantSDKManager 租户级 SDK 客户端管理器
// 支持按租户 ID + 链名称获取对应的 SDK 客户端，实现资源隔离
type TenantSDKManager struct {
	// tenantClients 租户级 SDK 客户端缓存: "tenantID:chainName" -> ChainSdkInterface
	tenantClients sync.Map

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
func NewTenantSDKManager(repo store.Repository, redisClient *commonEvent.RedisClient,
	logConf logx.LogConf, logger logx.Logger) *TenantSDKManager {
	return &TenantSDKManager{
		repo:        repo,
		redisClient: redisClient,
		logConf:     logConf,
		logger:      logger,
	}
}

// tenantChainKey 生成租户链缓存 key
func tenantChainKey(tenantID uint, chainName string) string {
	return fmt.Sprintf("%d:%s", tenantID, chainName)
}

// GetTenantSDKClient 获取租户级 SDK 客户端
// 如果缓存中存在则直接返回，否则从数据库加载配置并创建客户端
func (m *TenantSDKManager) GetTenantSDKClient(ctx context.Context, tenantID uint, chainName string) (ChainSdkInterface, error) {
	key := tenantChainKey(tenantID, chainName)

	// 尝试从缓存获取
	if client, ok := m.tenantClients.Load(key); ok {
		return client.(ChainSdkInterface), nil
	}

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

	// 缓存
	m.tenantClients.Store(key, client)
	m.logger.Infof("created tenant SDK client: tenant=%d, chain=%s", tenantID, chainName)

	return client, nil
}

// InvalidateTenantCache 使租户链配置缓存失效（配置变更时调用）
func (m *TenantSDKManager) InvalidateTenantCache(tenantID uint, chainName string) {
	key := tenantChainKey(tenantID, chainName)

	// 如果存在旧客户端，先停止
	if old, ok := m.tenantClients.LoadAndDelete(key); ok {
		if client, ok := old.(ChainSdkInterface); ok {
			client.Stop()
		}
	}

	m.logger.Infof("invalidated tenant SDK cache: tenant=%d, chain=%s", tenantID, chainName)
}

// InvalidateAllTenantCache 使某个租户的所有链配置缓存失效
func (m *TenantSDKManager) InvalidateAllTenantCache(tenantID uint) {
	prefix := fmt.Sprintf("%d:", tenantID)
	m.tenantClients.Range(func(key, value interface{}) bool {
		if k, ok := key.(string); ok && strings.HasPrefix(k, prefix) {
			if client, ok := value.(ChainSdkInterface); ok {
				client.Stop()
			}
			m.tenantClients.Delete(key)
		}
		return true
	})
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

// createSDKClient 根据链类型创建 SDK 客户端
func (m *TenantSDKManager) createSDKClient(ctx context.Context, chainType, chainName string,
	sdkConf *config.SdkConf, contractConfs map[string]*config.ContractConf) (ChainSdkInterface, error) {

	switch strings.ToLower(chainType) {
	case strings.ToLower(pb.ChainType_Ethereum.String()):
		client, err := NewEthereumClient(ctx, sdkConf.EthConf, contractConfs, m.redisClient)
		if err != nil {
			return nil, fmt.Errorf("create ethereum client for chain '%s': %w", chainName, err)
		}
		return client, nil

	case strings.ToLower(pb.ChainType_Chainmaker.String()):
		client, err := NewChainMakerClient(ctx, chainName, sdkConf.ConfFilePath,
			contractConfs, m.logConf, m.redisClient)
		if err != nil {
			return nil, fmt.Errorf("create chainmaker client for chain '%s': %w", chainName, err)
		}
		return client, nil

	case strings.ToLower(pb.ChainType_Solana.String()):
		client, err := NewSolanaClient(ctx, sdkConf.SolanaConf, contractConfs, m.redisClient)
		if err != nil {
			return nil, fmt.Errorf("create solana client for chain '%s': %w", chainName, err)
		}
		return client, nil

	default:
		return nil, fmt.Errorf("unsupported chain type: %s", chainType)
	}
}

// buildContractConfs 将数据库合约配置转换为内存配置格式
func buildContractConfs(dbConfigs []*store.TenantContractConfig) map[string]*config.ContractConf {
	result := make(map[string]*config.ContractConf)
	for _, dbConf := range dbConfigs {
		contractConf := &config.ContractConf{
			ContractName: dbConf.ContractName,
			ContractAddr: dbConf.ContractAddr,
			ContractType: dbConf.ContractType,
			Abi:          dbConf.AbiJSON,
		}

		// 解析额外配置
		if dbConf.ExtraConf != "" {
			_ = json.Unmarshal([]byte(dbConf.ExtraConf), contractConf)
			// 确保核心字段不被覆盖
			contractConf.ContractName = dbConf.ContractName
			contractConf.ContractAddr = dbConf.ContractAddr
			contractConf.ContractType = dbConf.ContractType
			if dbConf.AbiJSON != "" {
				contractConf.Abi = dbConf.AbiJSON
			}
		}

		result[dbConf.ContractName] = contractConf
	}
	return result
}
