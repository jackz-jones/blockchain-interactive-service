package service

import (
	"context"
	"fmt"
	"strings"
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

	// idxMu 保护 chainIndex：chainConfigID -> set of cacheKey
	// 用于精确按 chainConfigID 失效缓存，避免依赖 chainName（改名场景下会错过旧 name 的缓存）
	idxMu      sync.Mutex
	chainIndex map[uint]map[string]struct{}
}

// NewConfigResolver 创建配置解析器
func NewConfigResolver(repo store.Repository) *ConfigResolver {
	return &ConfigResolver{
		repo:       repo,
		chainIndex: make(map[uint]map[string]struct{}),
	}
}

// cacheKeyByName 生成缓存 key（按合约名称）
func cacheKeyByName(tenantID uint, chainName, contractName string) string {
	return fmt.Sprintf("%d:%s:%s", tenantID, chainName, contractName)
}

// cacheKeyByAddr 生成缓存 key（按合约地址）
func cacheKeyByAddr(tenantID uint, chainName, contractAddr string) string {
	return fmt.Sprintf("%d:%s:addr:%s", tenantID, chainName, contractAddr)
}

// addIndex 将 cacheKey 加入 chainConfigID 的索引
func (r *ConfigResolver) addIndex(chainConfigID uint, key string) {
	if chainConfigID == 0 {
		return
	}
	r.idxMu.Lock()
	defer r.idxMu.Unlock()
	set, ok := r.chainIndex[chainConfigID]
	if !ok {
		set = make(map[string]struct{})
		r.chainIndex[chainConfigID] = set
	}
	set[key] = struct{}{}
}

// popIndex 取出并清除指定 chainConfigID 的所有 cacheKey
func (r *ConfigResolver) popIndex(chainConfigID uint) []string {
	r.idxMu.Lock()
	defer r.idxMu.Unlock()
	set, ok := r.chainIndex[chainConfigID]
	if !ok {
		return nil
	}
	keys := make([]string, 0, len(set))
	for k := range set {
		keys = append(keys, k)
	}
	delete(r.chainIndex, chainConfigID)
	return keys
}

// removeIndexKey 从反向索引中删除单个 cacheKey
func (r *ConfigResolver) removeIndexKey(chainConfigID uint, key string) {
	r.idxMu.Lock()
	defer r.idxMu.Unlock()
	if set, ok := r.chainIndex[chainConfigID]; ok {
		delete(set, key)
		if len(set) == 0 {
			delete(r.chainIndex, chainConfigID)
		}
	}
}

// ResolveByName 通过链名 + 合约名解析到 DB 记录
func (r *ConfigResolver) ResolveByName(
	ctx context.Context, tenantID uint, chainName, contractName string,
) (*ConfigResolveResult, error) {
	key := cacheKeyByName(tenantID, chainName, contractName)

	if cached, ok := r.cache.Load(key); ok {
		return cached.(*ConfigResolveResult), nil
	}

	contract, err := r.repo.GetContractConfigByChainAndName(ctx, tenantID, chainName, contractName)
	if err != nil {
		return nil, fmt.Errorf("resolve contract config: %w", err)
	}
	if contract == nil {
		return nil, fmt.Errorf("未找到匹配的链配置或合约配置: chain=%s, contract=%s", chainName, contractName)
	}

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

	r.cache.Store(key, result)
	r.addIndex(contract.ChainConfigID, key)
	return result, nil
}

// ResolveByAddr 通过链名 + 合约地址解析到 DB 记录
func (r *ConfigResolver) ResolveByAddr(
	ctx context.Context, tenantID uint, chainName, contractAddr string,
) (*ConfigResolveResult, error) {
	key := cacheKeyByAddr(tenantID, chainName, contractAddr)

	if cached, ok := r.cache.Load(key); ok {
		return cached.(*ConfigResolveResult), nil
	}

	contract, err := r.repo.GetContractConfigByChainAndAddr(ctx, tenantID, chainName, contractAddr)
	if err != nil {
		return nil, fmt.Errorf("resolve contract config by addr: %w", err)
	}
	if contract == nil {
		return nil, fmt.Errorf("未找到匹配的链配置或合约配置: chain=%s, addr=%s", chainName, contractAddr)
	}

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

	r.cache.Store(key, result)
	r.addIndex(contract.ChainConfigID, key)
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

// InvalidateCache 清除指定租户 + chainName 的缓存
// 注意：chainName 改名场景请额外调用 InvalidateByChainConfigID
func (r *ConfigResolver) InvalidateCache(tenantID uint, chainName string) {
	prefix := fmt.Sprintf("%d:%s:", tenantID, chainName)
	r.cache.Range(func(key, value interface{}) bool {
		k, ok := key.(string)
		if !ok {
			return true
		}
		if !strings.HasPrefix(k, prefix) {
			return true
		}
		if v, ok := value.(*ConfigResolveResult); ok && v != nil {
			r.removeIndexKey(v.ChainConfigID, k)
		}
		r.cache.Delete(k)
		return true
	})
}

// InvalidateByChainConfigID 精确按 chainConfigID 清除缓存
// 适用于链配置改名 / 合约批量变更等无法仅靠 chainName 覆盖的场景
func (r *ConfigResolver) InvalidateByChainConfigID(chainConfigID uint) {
	keys := r.popIndex(chainConfigID)
	for _, k := range keys {
		r.cache.Delete(k)
	}
}

// InvalidateAll 清除所有缓存
func (r *ConfigResolver) InvalidateAll() {
	r.cache.Range(func(key, value interface{}) bool {
		r.cache.Delete(key)
		return true
	})
	r.idxMu.Lock()
	r.chainIndex = make(map[uint]map[string]struct{})
	r.idxMu.Unlock()
}
