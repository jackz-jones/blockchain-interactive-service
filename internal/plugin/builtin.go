package plugin

import (
	"context"
	"fmt"

	"github.com/jackz-jones/blockchain-interactive-service/internal/config"
	"github.com/jackz-jones/blockchain-interactive-service/internal/sdk"

	commonEvent "github.com/jackz-jones/common/event"
	"github.com/zeromicro/go-zero/core/logx"
)

// BuiltinPluginConf 内置插件配置（创建底层 SDK 客户端所需的完整配置）
type BuiltinPluginConf struct {
	ChainConf   *config.ChainConf
	LogConf     logx.LogConf
	RedisClient *commonEvent.RedisClient
	ChainName   string
}

// 内置插件版本号
const builtinPluginVersion = "1.0.0"

// ========== Ethereum 插件适配器 ==========

// EthereumPlugin 以太坊链插件
type EthereumPlugin struct {
	client sdk.ChainSdkInterface
	name   string
}

func NewEthereumPluginFactory() Factory {
	return func() ChainPlugin {
		return &EthereumPlugin{}
	}
}

func (p *EthereumPlugin) Name() string                     { return p.name }
func (p *EthereumPlugin) ChainType() string                { return "ethereum" }
func (p *EthereumPlugin) Version() string                  { return builtinPluginVersion }
func (p *EthereumPlugin) SDKClient() sdk.ChainSdkInterface { return p.client }

func (p *EthereumPlugin) Init(ctx context.Context, conf interface{}) error {
	c, ok := conf.(*BuiltinPluginConf)
	if !ok {
		return fmt.Errorf("invalid config type for ethereum plugin")
	}
	p.name = c.ChainName

	// 插件自己负责创建底层 SDK 客户端
	client, err := sdk.NewEthereumClient(ctx, c.ChainConf.SdkConf.EthConf,
		c.ChainConf.ContractConfs, c.RedisClient)
	if err != nil {
		return fmt.Errorf("create ethereum client: %w", err)
	}
	p.client = client
	return nil
}

func (p *EthereumPlugin) HealthCheck(ctx context.Context) error {
	if p.client == nil {
		return fmt.Errorf("ethereum client not initialized")
	}
	return nil
}

// ========== ChainMaker 插件适配器 ==========

// ChainMakerPlugin 长安链插件
type ChainMakerPlugin struct {
	client sdk.ChainSdkInterface
	name   string
}

func NewChainMakerPluginFactory() Factory {
	return func() ChainPlugin {
		return &ChainMakerPlugin{}
	}
}

func (p *ChainMakerPlugin) Name() string                     { return p.name }
func (p *ChainMakerPlugin) ChainType() string                { return "chainmaker" }
func (p *ChainMakerPlugin) Version() string                  { return builtinPluginVersion }
func (p *ChainMakerPlugin) SDKClient() sdk.ChainSdkInterface { return p.client }

func (p *ChainMakerPlugin) Init(ctx context.Context, conf interface{}) error {
	c, ok := conf.(*BuiltinPluginConf)
	if !ok {
		return fmt.Errorf("invalid config type for chainmaker plugin")
	}
	p.name = c.ChainName

	// 插件自己负责创建底层 SDK 客户端
	client, err := sdk.NewChainMakerClient(ctx, c.ChainName, c.ChainConf.SdkConf.ConfFilePath,
		c.ChainConf.ContractConfs, c.LogConf, c.RedisClient)
	if err != nil {
		return fmt.Errorf("create chainmaker client: %w", err)
	}
	p.client = client
	return nil
}

func (p *ChainMakerPlugin) HealthCheck(ctx context.Context) error {
	if p.client == nil {
		return fmt.Errorf("chainmaker client not initialized")
	}
	return nil
}

// ========== Solana 插件适配器 ==========

// SolanaPlugin Solana 链插件
type SolanaPlugin struct {
	client sdk.ChainSdkInterface
	name   string
}

func NewSolanaPluginFactory() Factory {
	return func() ChainPlugin {
		return &SolanaPlugin{}
	}
}

func (p *SolanaPlugin) Name() string                     { return p.name }
func (p *SolanaPlugin) ChainType() string                { return "solana" }
func (p *SolanaPlugin) Version() string                  { return builtinPluginVersion }
func (p *SolanaPlugin) SDKClient() sdk.ChainSdkInterface { return p.client }

func (p *SolanaPlugin) Init(ctx context.Context, conf interface{}) error {
	c, ok := conf.(*BuiltinPluginConf)
	if !ok {
		return fmt.Errorf("invalid config type for solana plugin")
	}
	p.name = c.ChainName

	// 插件自己负责创建底层 SDK 客户端
	client, err := sdk.NewSolanaClient(ctx, c.ChainConf.SdkConf.SolanaConf,
		c.ChainConf.ContractConfs, c.RedisClient)
	if err != nil {
		return fmt.Errorf("create solana client: %w", err)
	}
	p.client = client
	return nil
}

func (p *SolanaPlugin) HealthCheck(ctx context.Context) error {
	if p.client == nil {
		return fmt.Errorf("solana client not initialized")
	}
	return nil
}

// ========== 注册内置插件 ==========

// RegisterBuiltinPlugins 注册所有内置链插件工厂
func RegisterBuiltinPlugins(registry *Registry) {
	registry.RegisterFactory("ethereum", NewEthereumPluginFactory())
	registry.RegisterFactory("chainmaker", NewChainMakerPluginFactory())
	registry.RegisterFactory("solana", NewSolanaPluginFactory())
}
