package svc

import (
	"context"
	"fmt"
	"sync"

	"github.com/jackz-jones/blockchain-interactive-service/internal/config"
	"github.com/jackz-jones/blockchain-interactive-service/internal/sdk"

	commonEvent "github.com/jackz-jones/common/event"
	"github.com/zeromicro/go-zero/core/logx"
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

	// 初始化 redis client
	svc.initRedisClient()

	// 初始化已配置链的 sdk 客户端
	svc.initSdkClients()
	return svc
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
