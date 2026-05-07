package svc

import (
	"context"
	"fmt"
	"sync"

	"github.com/jackz-jones/blockchain-interactive-service/internal/config"
	"github.com/jackz-jones/blockchain-interactive-service/internal/sdk"
	"github.com/jackz-jones/blockchain-interactive-service/internal/store"
	"github.com/jackz-jones/blockchain-interactive-service/internal/tenant"

	commonEvent "github.com/jackz-jones/common/event"
	"github.com/zeromicro/go-zero/core/logx"
	"gorm.io/gorm"
)

type ServiceContext struct {
	Config config.Config

	// 与链交互的 sdk 客户端,chainName -> sdk.ChainSdkInterface
	SDKClients sync.Map
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

	// 初始化数据库
	svc.initDatabase()

	// 初始化 redis client
	svc.initRedisClient()

	// 初始化已配置链的 sdk 客户端
	svc.initSdkClients()
	return svc
}

// 初始化数据库连接
func (svc *ServiceContext) initDatabase() {
	dbConf := &store.DBConfig{
		Driver:   svc.Config.DatabaseConf.Driver,
		Host:     svc.Config.DatabaseConf.Host,
		Port:     svc.Config.DatabaseConf.Port,
		User:     svc.Config.DatabaseConf.User,
		Password: svc.Config.DatabaseConf.Password,
		DBName:   svc.Config.DatabaseConf.DBName,
		SSLMode:  svc.Config.DatabaseConf.SSLMode,
	}

	db, err := store.NewDB(dbConf)
	if err != nil {
		panic(fmt.Errorf("failed to connect database: %v", err))
	}
	svc.DB = db

	// 自动迁移表结构
	if svc.Config.DatabaseConf.AutoMigrate {
		if err := store.AutoMigrate(db); err != nil {
			panic(fmt.Errorf("failed to auto migrate database: %v", err))
		}
	}

	// 初始化 Repository 和 TenantService
	svc.Repo = store.NewGormRepository(db)
	svc.TenantService = tenant.NewService(svc.Repo)
}

// 初始化已配置链的 sdk 客户端
func (svc *ServiceContext) initSdkClients() {
	for chainConfName, chainConf := range svc.Config.ChainConfs {

		// 如果链未启用，则跳过
		if !chainConf.Enable {
			svc.Logger.Infof("chain %s is not enabled,skip...", chainConfName)
			continue
		}

		// 检查缓存中是否存在 sdk client，不存在会自动创建，并存入缓存
		_, err := sdk.GetSDKClient(svc.RootCtx, &svc.SDKClients, chainConfName, svc.Logger, chainConf,
			svc.Config.Log, svc.RedisClient)
		if err != nil {

			// 目前配置是确定的，如果出现错误，直接 panic
			panic(fmt.Errorf("failed to GetSDKClient for chain %s[%v] when initSdkClients,err: %v",
				chainConfName, chainConf, err))
		}
	}
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
