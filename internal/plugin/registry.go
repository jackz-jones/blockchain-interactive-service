package plugin

import (
	"context"
	"fmt"
	"sync"

	"github.com/jackz-jones/blockchain-interactive-service/internal/config"
	pb "github.com/jackz-jones/blockchain-interactive-service/pb"
	"github.com/zeromicro/go-zero/core/logx"
)

// ChainPlugin 链插件接口
// 所有链实现都必须实现此接口，以支持插件化注册和管理
type ChainPlugin interface {
	// Name 返回插件名称（唯一标识）
	Name() string

	// ChainType 返回链类型
	ChainType() string

	// Version 返回插件版本
	Version() string

	// Init 初始化插件
	Init(ctx context.Context, conf interface{}) error

	// HealthCheck 健康检查
	HealthCheck(ctx context.Context) error

	// CallContract 调用合约
	CallContract(methodType pb.MethodType, contractName, method string,
		args []*pb.KeyValuePair, txTimeout int64, withSyncResult bool) (string, string, error)

	// GetTxByTxId 查询交易
	GetTxByTxId(txId string) (string, bool, error)

	// SubscribeContractEvent 订阅合约事件
	SubscribeContractEvent(contractConf config.ContractConf, chainConfName, contractConfName,
		chainType, contractType string) error

	// Stop 停止插件，释放资源
	Stop() error
}

// PluginFactory 插件工厂函数类型
type PluginFactory func() ChainPlugin

// Registry 插件注册中心
type Registry struct {
	mu       sync.RWMutex
	plugins  map[string]ChainPlugin  // 已实例化的插件: name -> plugin
	factories map[string]PluginFactory // 插件工厂: chainType -> factory
	logger   logx.Logger
}

// NewRegistry 创建插件注册中心
func NewRegistry(logger logx.Logger) *Registry {
	return &Registry{
		plugins:   make(map[string]ChainPlugin),
		factories: make(map[string]PluginFactory),
		logger:    logger,
	}
}

// RegisterFactory 注册插件工厂（按链类型注册）
// 开发者实现新链插件后，调用此方法注册工厂函数
func (r *Registry) RegisterFactory(chainType string, factory PluginFactory) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.factories[chainType] = factory
	r.logger.Infof("[Plugin] registered factory for chain type: %s", chainType)
}

// CreatePlugin 通过工厂创建插件实例
func (r *Registry) CreatePlugin(ctx context.Context, name, chainType string, conf interface{}) (ChainPlugin, error) {
	r.mu.RLock()
	factory, exists := r.factories[chainType]
	r.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("no plugin factory registered for chain type: %s", chainType)
	}

	plugin := factory()
	if err := plugin.Init(ctx, conf); err != nil {
		return nil, fmt.Errorf("init plugin '%s' (type=%s): %w", name, chainType, err)
	}

	r.mu.Lock()
	r.plugins[name] = plugin
	r.mu.Unlock()

	r.logger.Infof("[Plugin] created plugin: name=%s, type=%s, version=%s",
		name, chainType, plugin.Version())

	return plugin, nil
}

// GetPlugin 获取已注册的插件实例
func (r *Registry) GetPlugin(name string) (ChainPlugin, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	plugin, exists := r.plugins[name]
	return plugin, exists
}

// ListPlugins 列出所有已注册的插件
func (r *Registry) ListPlugins() []PluginInfo {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var infos []PluginInfo
	for name, plugin := range r.plugins {
		infos = append(infos, PluginInfo{
			Name:      name,
			ChainType: plugin.ChainType(),
			Version:   plugin.Version(),
		})
	}
	return infos
}

// ListSupportedChainTypes 列出所有已注册工厂支持的链类型
func (r *Registry) ListSupportedChainTypes() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var types []string
	for chainType := range r.factories {
		types = append(types, chainType)
	}
	return types
}

// RemovePlugin 移除并停止插件
func (r *Registry) RemovePlugin(name string) error {
	r.mu.Lock()
	plugin, exists := r.plugins[name]
	if exists {
		delete(r.plugins, name)
	}
	r.mu.Unlock()

	if !exists {
		return fmt.Errorf("plugin '%s' not found", name)
	}

	if err := plugin.Stop(); err != nil {
		r.logger.Errorf("[Plugin] stop plugin '%s' error: %v", name, err)
		return err
	}

	r.logger.Infof("[Plugin] removed plugin: %s", name)
	return nil
}

// StopAll 停止所有插件
func (r *Registry) StopAll() {
	r.mu.Lock()
	defer r.mu.Unlock()

	for name, plugin := range r.plugins {
		if err := plugin.Stop(); err != nil {
			r.logger.Errorf("[Plugin] stop plugin '%s' error: %v", name, err)
		}
	}
	r.plugins = make(map[string]ChainPlugin)
	r.logger.Info("[Plugin] all plugins stopped")
}

// HealthCheckAll 对所有插件执行健康检查
func (r *Registry) HealthCheckAll(ctx context.Context) map[string]error {
	r.mu.RLock()
	defer r.mu.RUnlock()

	results := make(map[string]error)
	for name, plugin := range r.plugins {
		results[name] = plugin.HealthCheck(ctx)
	}
	return results
}

// PluginInfo 插件信息
type PluginInfo struct {
	Name      string `json:"name"`
	ChainType string `json:"chain_type"`
	Version   string `json:"version"`
}
