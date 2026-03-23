package sdk

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/jackz-jones/blockchain-interactive-service/internal/config"
	pb "github.com/jackz-jones/blockchain-interactive-service/pb"

	"chainmaker.org/chainmaker/common/v2/log"
	commonEvent "github.com/jackz-jones/common/event"
	"github.com/zeromicro/go-zero/core/logx"
	"go.uber.org/zap"
)

var (

	// SubscribeFlag 全局订阅标记: chainConfName + contractConfName -> true
	SubscribeFlag = sync.Map{}
)

// GetSDKClient 获取可用的 sdk client
func GetSDKClient(sdkClients *sync.Map, chainConfName string, logger logx.Logger, chainConf *config.ChainConf,
	logConf logx.LogConf, redisClient *commonEvent.RedisClient) (sdkClient ChainSdkInterface, err error) {

	// 判断是否启用
	if !chainConf.Enable {
		return nil, fmt.Errorf("%s is not enabled, please check the chain config", chainConfName)
	}

	// 尝试从缓存中获取
	loadedSDK, ok := sdkClients.Load(chainConfName)
	if ok {
		sdkClient, ok = loadedSDK.(ChainSdkInterface)
		if !ok {
			return nil, fmt.Errorf("loaded for %s is not ChainSdkInterface", chainConfName)
		}

		logger.Info("success to get sdk client from cache")
		return sdkClient, nil
	}

	logger.Infof("no sdk client for %s from cache,should create", chainConfName)

	// 重建 sdk 客户端
	switch strings.ToLower(chainConf.ChainType) {
	case strings.ToLower(pb.ChainType_Ethereum.String()):
		ethClient, err2 := NewEthereumClient(context.Background(), chainConf.SdkConf.EthConf,
			chainConf.ContractConfs, redisClient)
		if err2 != nil {
			logger.Errorf("failed to NewEthereumClient: %v", err2)
			return nil, fmt.Errorf("failed to NewEthereumClient for %s: %v", chainConfName, err2)
		}

		// 缓存以太坊 sdk client
		sdkClients.Store(chainConfName, ethClient)
		logger.Infof("success to create ethereum sdk client for %s", chainConfName)
		return ethClient, nil

	case strings.ToLower(pb.ChainType_Chainmaker.String()):
		chainMakerClient, err2 := NewChainMakerClient(context.Background(), chainConfName,
			chainConf.SdkConf.ConfFilePath, chainConf.ContractConfs, logConf, redisClient)
		if err2 != nil {
			logger.Errorf("failed to NewChainMakerClient: %v", err2)
			return nil, fmt.Errorf("failed to NewChainMakerClient for %s: %v", chainConfName, err2)
		}

		// 缓存长安链 sdk client
		sdkClients.Store(chainConfName, chainMakerClient)
		logger.Infof("success to create chainmaker sdk client for %s", chainConfName)
		return chainMakerClient, nil

	default:
		logger.Errorf("unknown chain type: %s", chainConf.ChainType)
		return nil, fmt.Errorf("unknown chain type: %s, please check chain config for %s", chainConf.ChainType, chainConfName)
	}
}

// StopAllSdkClients 释放所有 sdk 资源
func StopAllSdkClients(sdkClients *sync.Map, logger logx.Logger) {
	sdkClients.Range(func(key, value any) bool {
		// 获取 chainConfName
		chainConfName, ok := key.(string)
		if !ok {
			logger.Errorf("key[%v] is not string in ctx.SDKClients", key)
		}

		// 获取 sdk client
		sdkClient, ok := value.(ChainSdkInterface)
		if !ok {
			logger.Errorf("value[%v] is not ChainSdkInterface in ctx.SDKClients", value)
			return true
		}

		// 停止 sdk client
		err := sdkClient.Stop()
		if err != nil {
			logger.Errorf("failed to stop sdkClient for chain %s before exit: %v", chainConfName, err)
		}

		return true
	})
}

// StartSubscribe 启动订阅
func StartSubscribe(conf config.Config, sdkClients *sync.Map, logger logx.Logger,
	redisClient *commonEvent.RedisClient) {
	go func() {
		for {
			for chainConfName, chainConf := range conf.ChainConfs {

				// 如果链未启用，则跳过
				if !chainConf.Enable {
					continue
				}

				// 从缓存中获取 sdk client
				sdkClient, err := GetSDKClient(sdkClients, chainConfName, logger, chainConf, conf.Log, redisClient)
				if err != nil {
					logger.Errorf("failed to GetSDKClient for chain %s before subscribe contract event,err: %v",
						chainConfName, err)
					continue
				}

				// 订阅多个合约事件
				for contractConfName, contractConf := range chainConf.ContractConfs {

					// 是否要订阅合约事件
					if !contractConf.EnableSubscribe {
						continue
					}

					cc := contractConf
					go func(chainConfName, contractConfName, chainType, contractType string,
						subscribeConf config.SubscribeConf, sdkClient ChainSdkInterface, logger logx.Logger) {

						// 检查是否重复订阅
						val, ok := SubscribeFlag.Load(fmt.Sprintf("%s-%s", chainConfName, contractConfName))
						if ok && val.(bool) {
							logger.Infof("[chain: %s] [contract: %s] already subscribed", chainConfName, contractConfName)
							return
						}

						// 否则标记为已订阅
						SubscribeFlag.Store(fmt.Sprintf("%s-%s", chainConfName, contractConfName), true)

						// 开始订阅
						err = sdkClient.SubscribeContractEvent(*cc, chainConfName, contractConfName, chainType, contractType)
						if err != nil {
							logger.Errorf("failed to subscribe chain %s contract %s event,err: %v", chainConfName, contractConfName, err)

							// 订阅失败，则取消标记
							SubscribeFlag.Store(fmt.Sprintf("%s-%s", chainConfName, contractConfName), false)
						}
					}(chainConfName, contractConfName, strings.ToLower(chainConf.ChainType),
						strings.ToLower(contractConf.ContractType), conf.SubscribeConf, sdkClient, logger)
				}
			}

			// 每隔3秒重新检查一下所有链的订阅
			time.Sleep(time.Second * 3)
		}
	}()
}

func GetDefaultSdkLogger(logPath string, maxAge int) *zap.SugaredLogger {
	logConfig := log.LogConfig{
		Module:       "[ChainMaker SDK]",
		LogPath:      logPath,
		LogLevel:     log.LEVEL_INFO,
		MaxAge:       maxAge,
		JsonFormat:   false,
		ShowLine:     true,
		LogInConsole: false,
	}

	logger, _ := log.InitSugarLogger(&logConfig)
	return logger
}
