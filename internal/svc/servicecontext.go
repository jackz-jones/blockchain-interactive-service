package svc

import (
	"context"
	"fmt"
	"net/http"
	"os"

	"github.com/jackz-jones/blockchain-interactive-service/internal/billing"
	"github.com/jackz-jones/blockchain-interactive-service/internal/config"
	"github.com/jackz-jones/blockchain-interactive-service/internal/middleware"
	"github.com/jackz-jones/blockchain-interactive-service/internal/plugin"
	"github.com/jackz-jones/blockchain-interactive-service/internal/sdk"
	"github.com/jackz-jones/blockchain-interactive-service/internal/service"
	"github.com/jackz-jones/blockchain-interactive-service/internal/store"
	"github.com/jackz-jones/blockchain-interactive-service/internal/tenant"
	"github.com/robfig/cron/v3"

	commonEvent "github.com/jackz-jones/common/event"
	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/rest"
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

	// 定时任务调度器
	CronScheduler *cron.Cron

	// 配置解析器（业务语义到 ID 的映射层）
	ConfigResolver *service.ConfigResolver

	// ChainClientFactory 链客户端工厂函数，通过它创建各链 SDK 客户端
	ChainClientFactory sdk.ChainClientFactory

	// HTTP 中间件（供 goctl 生成的路由使用）
	AuthMiddleware      rest.Middleware
	AuditMiddleware     rest.Middleware
	RateLimitMiddleware rest.Middleware
	QuotaMiddleware     rest.Middleware

	// APIKeyAuthCache API Key 认证结果缓存（HTTP + gRPC 共用）
	APIKeyAuthCache *middleware.APIKeyAuthCache
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

	// 初始化 HTTP 中间件
	svc.initHTTPMiddlewares()

	return svc
}

// 初始化数据库连接
func (svc *ServiceContext) initDatabase() {
	db, err := store.NewDB(&svc.Config.DatabaseConf)
	if err != nil {
		// 记录关键错误后以退出码 1 结束进程，允许 defer/日志 flush 正常执行
		// 相比 panic 更利于容器编排系统识别退出状态
		logx.Errorf("failed to init database: %v", err)
		os.Exit(1)
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

	// 初始化定时任务调度器
	svc.initCronScheduler()

	// 初始化配置解析器
	svc.ConfigResolver = service.NewConfigResolver(svc.Repo)
}

// 初始化redis client
func (svc *ServiceContext) initRedisClient() {
	redisClient, err := commonEvent.NewRedisClient(svc.Config.SubscribeConf.ConfType, svc.Config.SubscribeConf.RedisAddr,
		svc.Config.SubscribeConf.RedisUserName, svc.Config.SubscribeConf.RedisPassword, svc.Config.SubscribeConf.MasterName)
	if err != nil {
		// 记录关键错误后以退出码 1 结束进程，允许 defer/日志 flush 正常执行
		logx.Errorf("failed to NewRedisClient, conf: %v, err: %v", svc.Config.SubscribeConf, err)
		os.Exit(1)
	}

	svc.RedisClient = redisClient
}

// initCronScheduler 初始化定时任务调度器
func (svc *ServiceContext) initCronScheduler() {
	svc.CronScheduler = cron.New(cron.WithSeconds())

	billingSvc := svc.BillingService
	logger := svc.Logger

	// 每日24:00生成日账单
	_, err := svc.CronScheduler.AddFunc("0 0 0 * * *", func() {
		ctx := context.Background()
		logger.Info("[Cron] starting daily bill generation")
		if err := billingSvc.GenerateDailyBills(ctx); err != nil {
			logger.Errorf("[Cron] daily bill generation failed: %v", err)
		}
	})
	if err != nil {
		logger.Errorf("[Cron] failed to register daily bill job: %v", err)
	}

	// 每月1日00:05生成月度汇总账单
	_, err = svc.CronScheduler.AddFunc("0 5 0 1 * *", func() {
		ctx := context.Background()
		logger.Info("[Cron] starting monthly bill generation")
		if err := billingSvc.GenerateMonthlyBills(ctx); err != nil {
			logger.Errorf("[Cron] monthly bill generation failed: %v", err)
		}
	})
	if err != nil {
		logger.Errorf("[Cron] failed to register monthly bill job: %v", err)
	}

	// 每月1日00:00重置月度计数器
	_, err = svc.CronScheduler.AddFunc("0 0 0 1 * *", func() {
		ctx := context.Background()
		logger.Info("[Cron] resetting monthly counters")
		if err := billingSvc.ResetMonthlyCounters(ctx); err != nil {
			logger.Errorf("[Cron] monthly counter reset failed: %v", err)
		}
	})
	if err != nil {
		logger.Errorf("[Cron] failed to register monthly counter reset job: %v", err)
	}

	svc.CronScheduler.Start()
	logger.Info("[Cron] scheduler started with daily/monthly billing jobs")
}

// initHTTPMiddlewares 初始化 HTTP 中间件（适配 rest.Middleware 签名）
func (svc *ServiceContext) initHTTPMiddlewares() {
	// API Key 认证结果缓存（TTL 60s，last_used 节流 60s，负缓存 5s）
	svc.APIKeyAuthCache = middleware.NewAPIKeyAuthCache(middleware.DefaultAPIKeyCacheConfig())

	// 认证中间件
	authMw := middleware.HTTPAuthMiddleware(svc.Repo, svc.APIKeyAuthCache)
	svc.AuthMiddleware = toRestMiddleware(authMw)

	// 审计中间件（位于认证之后、限流之前）
	auditMw := middleware.HTTPAuditMiddleware(svc.Repo)
	svc.AuditMiddleware = toRestMiddleware(auditMw)

	// 限流中间件
	rateLimiter := middleware.NewRateLimiter(svc.Config.GatewayConf.RateLimit)
	rateLimitMw := middleware.HTTPRateLimitMiddleware(rateLimiter)
	svc.RateLimitMiddleware = toRestMiddleware(rateLimitMw)

	// 配额中间件
	quotaMw := middleware.HTTPQuotaMiddleware(svc.BillingService)
	svc.QuotaMiddleware = toRestMiddleware(quotaMw)
}

// toRestMiddleware 将 func(http.Handler) http.Handler 适配为 rest.Middleware
func toRestMiddleware(mw func(http.Handler) http.Handler) rest.Middleware {
	return func(next http.HandlerFunc) http.HandlerFunc {
		return mw(next).ServeHTTP
	}
}
