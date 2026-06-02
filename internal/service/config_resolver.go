package service

import (
	"context"
	"fmt"
	"sync"

	"github.com/jackz-jones/blockchain-interactive-service/internal/store"
)

// ConfigResolveResult 配置解析结果
type ConfigResolveResult struct {
	ChainConfigID    uint
	ContractConfigID uint
	ChainName        string
	ContractName     string
	ContractAddr     string
	ChainType        string
}

// ConfigResolver 业务语义到 ID 的映射层
// 上层业务通过链名 + 合约名/合约地址定位到具体的 DB 记录（chainConfigID + contractConfigID）
type ConfigResolver struct {
	repo store.Repository
	// key: "tenantID:chainName:contractName" or "tenantID:chainName:addr:contractAddr"
	cache sync.Map
}

// NewConfigResolver 创建配置解析器
func NewConfigResolver(repo store.Repository) *ConfigResolver {
	return &ConfigResolver{
		repo: repo,
	}
}

// cacheKey 生成缓存 key（按合约名称）
func cacheKeyByName(tenantID uint, chainName, contractName string) string {
	return fmt.Sprintf("%d:%s:%s", tenantID, chainName, contractName)
}

// cacheKeyByAddr 生成缓存 key（按合约地址）
func cacheKeyByAddr(tenantID uint, chainName, contractAddr string) string {
	return fmt.Sprintf("%d:%s:addr:%s", tenantID, chainName, contractAddr)
}

// ResolveByName 通过链名 + 合约名解析到 DB 记录
func (r *ConfigResolver) ResolveByName(
	ctx context.Context, tenantID uint, chainName, contractName string,
) (*ConfigResolveResult, error) {
	key := cacheKeyByName(tenantID, chainName, contractName)

	// 尝试从缓存获取
	if cached, ok := r.cache.Load(key); ok {
		return cached.(*ConfigResolveResult), nil
	}

	// 从数据库查询
	contract, err := r.repo.GetContractConfigByChainAndName(ctx, tenantID, chainName, contractName)
	if err != nil {
		return nil, fmt.Errorf("resolve contract config: %w", err)
	}
	if contract == nil {
		return nil, fmt.Errorf("未找到匹配的链配置或合约配置: chain=%s, contract=%s", chainName, contractName)
	}

	// 获取链配置信息
	chainConfig, err := r.repo.GetChainConfigByID(ctx, contract.ChainConfigID)
	if err != nil {
		return nil, fmt.Errorf("get chain config: %w", err)
	}

	result := &ConfigResolveResult{
		ChainConfigID:    contract.ChainConfigID,
		ContractConfigID: contract.ID,
		ChainName:        chainName,
		ContractName:     contractName,
		ContractAddr:     contract.ContractAddr,
		ChainType:        chainConfig.ChainType,
	}

	// 缓存结果
	r.cache.Store(key, result)
	return result, nil
}

// ResolveByAddr 通过链名 + 合约地址解析到 DB 记录
func (r *ConfigResolver) ResolveByAddr(
	ctx context.Context, tenantID uint, chainName, contractAddr string,
) (*ConfigResolveResult, error) {
	key := cacheKeyByAddr(tenantID, chainName, contractAddr)

	// 尝试从缓存获取
	if cached, ok := r.cache.Load(key); ok {
		return cached.(*ConfigResolveResult), nil
	}

	// 从数据库查询
	contract, err := r.repo.GetContractConfigByChainAndAddr(ctx, tenantID, chainName, contractAddr)
	if err != nil {
		return nil, fmt.Errorf("resolve contract config by addr: %w", err)
	}
	if contract == nil {
		return nil, fmt.Errorf("未找到匹配的链配置或合约配置: chain=%s, addr=%s", chainName, contractAddr)
	}

	// 获取链配置信息
	chainConfig, err := r.repo.GetChainConfigByID(ctx, contract.ChainConfigID)
	if err != nil {
		return nil, fmt.Errorf("get chain config: %w", err)
	}

	result := &ConfigResolveResult{
		ChainConfigID:    contract.ChainConfigID,
		ContractConfigID: contract.ID,
		ChainName:        chainName,
		ContractName:     contract.ContractName,
		ContractAddr:     contractAddr,
		ChainType:        chainConfig.ChainType,
	}

	// 缓存结果
	r.cache.Store(key, result)
	return result, nil
}

// Resolve 通过链名 + 合约名或合约地址解析（优先按名称，其次按地址）
func (r *ConfigResolver) Resolve(
	ctx context.Context, tenantID uint, chainName, contractName, contractAddr string,
) (*ConfigResolveResult, error) {
	if contractName != "" {
		return r.ResolveByName(ctx, tenantID, chainName, contractName)
	}
	if contractAddr != "" {
		return r.ResolveByAddr(ctx, tenantID, chainName, contractAddr)
	}
	return nil, fmt.Errorf("contract_name 或 contract_addr 至少需要提供一个")
}

// InvalidateCache 清除指定租户的缓存（配置变更时调用）
func (r *ConfigResolver) InvalidateCache(tenantID uint, chainName string) {
	prefix := fmt.Sprintf("%d:%s:", tenantID, chainName)
	r.cache.Range(func(key, value interface{}) bool {
		if k, ok := key.(string); ok && len(k) >= len(prefix) && k[:len(prefix)] == prefix {
			r.cache.Delete(key)
		}
		return true
	})
}

// InvalidateAll 清除所有缓存
func (r *ConfigResolver) InvalidateAll() {
	r.cache.Range(func(key, value interface{}) bool {
		r.cache.Delete(key)
		return true
	})
}
