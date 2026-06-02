package svc

import (
	"context"
	"fmt"

	"github.com/jackz-jones/blockchain-interactive-service/internal/billing"
	"github.com/jackz-jones/blockchain-interactive-service/internal/config"
	"github.com/jackz-jones/blockchain-interactive-service/internal/plugin"
	"github.com/jackz-jones/blockchain-interactive-service/internal/sdk"
	"github.com/jackz-jones/blockchain-interactive-service/internal/service"
	"github.com/jackz-jones/blockchain-interactive-service/internal/store"
	"github.com/jackz-jones/blockchain-interactive-service/internal/tenant"

	commonEvent "github.com/jackz-jones/common/event"
	"github.com/zeromicro/go-zero/core/logx"
	"gorm.io/gorm"
)

type ServiceContext struct {
	Config config.Config
	logx.Logger

	// RootCtx 服务级根 ctx，所有 SDK 客户端共享该 ctx 作为父 ctx；
	// 进程退出前调用 Cancel() 以通知订阅 goroutine 等退出。
	RootCtx context.Context
	Cancel  context.CancelFunc

	// redis client
	RedisClient *commonEvent.RedisClient

	// 数据库连接
	DB *gorm.DB

	// 数据访问层
	Repo store.Repository

	// 租户管理服务
	TenantService *tenant.Service

	// 租户级 SDK 客户端管理器（支持按租户隔离的链配置动态加载）
	TenantSDKManager *sdk.TenantSDKManager

	// 计费服务
	BillingService *billing.Service

	// 配置解析器（业务语义到 ID 的映射层）
	ConfigResolver *service.ConfigResolver

	// ChainClientFactory 链客户端工厂函数，通过它创建各链 SDK 客户端
	ChainClientFactory sdk.ChainClientFactory
}

func NewServiceContext(c config.Config) *ServiceContext {
	logx.MustSetup(c.Log)

	rootCtx, cancel := context.WithCancel(context.Background())
	svc := &ServiceContext{
		Config:  c,
		Logger:  logx.WithContext(rootCtx),
		RootCtx: rootCtx,
		Cancel:  cancel,
	}

	// 初始化插件注册中心，注册内置链插件工厂
	pluginRegistry := plugin.NewRegistry(svc.Logger)
	plugin.RegisterBuiltinPlugins(pluginRegistry)

	// 创建链客户端工厂函数，通过插件注册中心创建各链 SDK 客户端
	svc.ChainClientFactory = func(ctx context.Context, chainName, chainType string,
		chainConf *sdk.ChainConf, logConf logx.LogConf,
		redisClient *commonEvent.RedisClient) (sdk.ChainSdkInterface, error) {

		// 构造插件初始化所需的配置
		pluginConf := &plugin.BuiltinPluginConf{
			ChainConf:   chainConf,
			LogConf:     logConf,
			RedisClient: redisClient,
			ChainName:   chainName,
		}

		// 通过插件注册中心创建插件实例（工厂模式）
		p, err := pluginRegistry.CreatePlugin(ctx, chainName, chainType, pluginConf)
		if err != nil {
			return nil, fmt.Errorf("create %s plugin: %w", chainType, err)
		}

		return p.SDKClient(), nil
	}

	// 初始化 redis client（需要在 initDatabase 之前，因为 TenantSDKManager 依赖 RedisClient）
	svc.initRedisClient()

	// 初始化数据库
	svc.initDatabase()

	return svc
}

// 初始化数据库连接
func (svc *ServiceContext) initDatabase() {
	db, err := store.NewDB(&svc.Config.DatabaseConf)
	if err != nil {
		panic(fmt.Errorf("failed to init database: %v", err))
	}
	svc.DB = db

	// 初始化 Repository 和 TenantService
	svc.Repo = store.NewGormRepository(db)
	svc.TenantService = tenant.NewService(svc.Repo)

	// 初始化租户级 SDK 管理器（传入链客户端工厂函数，由插件创建客户端）
	svc.TenantSDKManager = sdk.NewTenantSDKManager(
		svc.Repo, svc.ChainClientFactory, svc.RedisClient, svc.Config.Log, svc.Logger)

	// 初始化计费服务
	svc.BillingService = billing.NewService(svc.Repo, svc.Logger)

	// 初始化配置解析器
	svc.ConfigResolver = service.NewConfigResolver(svc.Repo)
}

// 初始化redis client
func (svc *ServiceContext) initRedisClient() {
	redisClient, err := commonEvent.NewRedisClient(svc.Config.SubscribeConf.ConfType, svc.Config.SubscribeConf.RedisAddr,
		svc.Config.SubscribeConf.RedisUserName, svc.Config.SubscribeConf.RedisPassword, svc.Config.SubscribeConf.MasterName)
	if err != nil {

		// 目前配置是确定的，如果出现错误，直接 panic
		panic(fmt.Errorf("failed to NewRedisClient,conf: %v,err: %v", svc.Config.SubscribeConf, err))
	}

	svc.RedisClient = redisClient
}
