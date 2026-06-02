package sdk

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/jackz-jones/blockchain-interactive-service/internal/config"
	"github.com/jackz-jones/blockchain-interactive-service/internal/store"

	"chainmaker.org/chainmaker/common/v2/log"
	commonEvent "github.com/jackz-jones/common/event"
	"github.com/zeromicro/go-zero/core/logx"
	"go.uber.org/zap"
)

// ChainClientFactory 链客户端工厂函数类型
// 根据链名称、链类型和配置创建 ChainSdkInterface 实例
// 引入此抽象层使 sdk 包不直接依赖 plugin 包，避免循环依赖
type ChainClientFactory func(ctx context.Context, chainName, chainType string,
	chainConf *config.ChainConf, logConf logx.LogConf, redisClient *commonEvent.RedisClient) (ChainSdkInterface, error)

var (

	// SubscribeFlag 全局订阅标记: chainConfName + contractConfName -> true
	SubscribeFlag = sync.Map{}
)

// stopAllSdkClientsTimeout 优雅停止所有 SDK 客户端的超时时间
const stopAllSdkClientsTimeout = 10 * time.Second

// subscribeRescheduleInterval 订阅重调度间隔，订阅 goroutine 异常退出后在此间隔触发重订阅
const subscribeRescheduleInterval = 3 * time.Second

// BuildSubscribeLogFields 根据任意字段构建 logx.LogField 列表，供各 SDK 的 SubscribeContractEvent 使用。
// 这里统一做 map -> []logx.LogField 的转换，消除各 SDK 中重复的样板代码。
func BuildSubscribeLogFields(fields map[string]interface{}) []logx.LogField {
	logFields := make([]logx.LogField, 0, len(fields))
	for k, v := range fields {
		logFields = append(logFields, logx.Field(k, v))
	}
	return logFields
}

// subscribeKey 生成 SubscribeFlag 使用的 key
func subscribeKey(chainConfName, contractConfName string) string {
	return fmt.Sprintf("%s-%s", chainConfName, contractConfName)
}

// subscribeKeyByID 生成基于 DB ID 的 SubscribeFlag key（DB 路径专用）
func subscribeKeyByID(chainConfigID, contractConfigID uint) string {
	return fmt.Sprintf("db:%d-%d", chainConfigID, contractConfigID)
}

// GetSDKClient 获取可用的 sdk client
// 优先从 sdkClients 缓存获取；缓存未命中时通过 factory 创建链客户端并存入缓存
func GetSDKClient(ctx context.Context, sdkClients *sync.Map, chainConfName string, logger logx.Logger,
	chainConf *config.ChainConf, logConf logx.LogConf, redisClient *commonEvent.RedisClient,
	factory ChainClientFactory) (sdkClient ChainSdkInterface, err error) {

	// 判断是否启用
	if !chainConf.Enable {
		return nil, fmt.Errorf("%s is not enabled, please check the chain config", chainConfName)
	}

	// 尝试从缓存中获取
	if sdkClient, ok := loadSDKClient(sdkClients, chainConfName); ok {
		logger.Info("success to get sdk client from cache")
		return sdkClient, nil
	}

	logger.Infof("no sdk client for %s from cache,should create", chainConfName)

	// 通过工厂函数创建链客户端
	chainType := strings.ToLower(chainConf.ChainType)
	sdkClient, err = factory(ctx, chainConfName, chainType, chainConf, logConf, redisClient)
	if err != nil {
		logger.Errorf("failed to create sdk client for chain type %s: %v", chainType, err)
		return nil, fmt.Errorf("failed to create sdk client for %s: %v", chainConfName, err)
	}

	sdkClients.Store(chainConfName, sdkClient)
	logger.Infof("success to create %s sdk client for %s via chain client factory", chainType, chainConfName)
	return sdkClient, nil
}

// loadSDKClient 类型安全地从 sync.Map 读取 ChainSdkInterface
func loadSDKClient(sdkClients *sync.Map, chainConfName string) (ChainSdkInterface, bool) {
	v, ok := sdkClients.Load(chainConfName)
	if !ok {
		return nil, false
	}
	client, ok := v.(ChainSdkInterface)
	return client, ok
}

// StopAllSdkClients 并发释放所有 sdk 资源，整体设置 stopAllSdkClientsTimeout 秒超时，
// 超时后强制返回，避免某条链 Stop 卡住阻塞整个进程退出。
func StopAllSdkClients(sdkClients *sync.Map, logger logx.Logger) {
	var wg sync.WaitGroup
	sdkClients.Range(func(key, value any) bool {
		chainConfName, _ := key.(string)
		sdkClient, ok := value.(ChainSdkInterface)
		if !ok {
			logger.Errorf("value[%v] is not ChainSdkInterface in ctx.SDKClients", value)
			return true
		}

		wg.Add(1)
		go func(name string, client ChainSdkInterface) {
			defer wg.Done()
			if err := client.Stop(); err != nil {
				logger.Errorf("failed to stop sdkClient for chain %s before exit: %v", name, err)
			}
		}(chainConfName, sdkClient)
		return true
	})

	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		logger.Info("all sdk clients stopped gracefully")
	case <-time.After(stopAllSdkClientsTimeout):
		logger.Errorf("stop all sdk clients timeout after %s, force return", stopAllSdkClientsTimeout)
	}
}

// StartSubscribe 启动订阅
// 行为说明：
//  1. 主 goroutine 每 subscribeRescheduleInterval 轮询一次所有链的所有合约。
//  2. 对已启用订阅但 SubscribeFlag 为 false/不存在的合约，启动订阅 goroutine。
//  3. 订阅 goroutine 内部使用 **局部 err** 变量，消除与外层变量的数据竞争。
//  4. 订阅 goroutine 退出时通过 defer 清理 SubscribeFlag，使下一轮可重订阅。
//  5. 主 goroutine 监听 ctx.Done，收到退出信号后立即返回；订阅 goroutine 由 SDK 的 Stop()/ctx
//     取消机制驱动退出。
func StartSubscribe(ctx context.Context, conf config.Config, sdkClients *sync.Map, logger logx.Logger,
	redisClient *commonEvent.RedisClient, factory ChainClientFactory,
	tenantMgr *TenantSDKManager, repo store.Repository) {
	go func() {
		ticker := time.NewTicker(subscribeRescheduleInterval)
		defer ticker.Stop()
		for {
			// 配置文件路径订阅
			scheduleOnce(ctx, conf, sdkClients, logger, redisClient, factory)

			// DB 配置路径订阅
			if repo != nil && tenantMgr != nil {
				scheduleDBOnce(ctx, tenantMgr, repo, logger, redisClient, factory, conf.Log)
			}

			// 每隔 subscribeRescheduleInterval 重新检查一下所有链的订阅，或被 ctx 中断退出
			select {
			case <-ctx.Done():
				logger.Info("StartSubscribe root ctx done, exiting reschedule loop")
				return
			case <-ticker.C:
			}
		}
	}()
}

// scheduleOnce 执行一次 "扫描所有链/合约并拉起订阅" 的过程
func scheduleOnce(ctx context.Context, conf config.Config, sdkClients *sync.Map, logger logx.Logger,
	redisClient *commonEvent.RedisClient, factory ChainClientFactory) {
	for chainConfName, chainConf := range conf.ChainConfs {

		// 如果链未启用，则跳过
		if !chainConf.Enable {
			continue
		}

		// 从缓存中获取 sdk client
		sdkClient, err := GetSDKClient(ctx, sdkClients, chainConfName, logger, chainConf, conf.Log, redisClient, factory)
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
			chainType := strings.ToLower(chainConf.ChainType)

			key := subscribeKey(chainConfName, contractConfName)
			go runSubscribeOnce(sdkClient, cc, chainConfName, contractConfName, chainType, logger, 0, 0, key)
		}
	}
}

// runSubscribeOnce 在独立 goroutine 中执行一次订阅。
// - 使用局部 subErr 变量，不与外层共享。
// - defer 中清理 SubscribeFlag，使得下一次轮询可重订阅。
// - flagKey 由调用方传入，支持配置文件路径和 DB 路径两种 key 格式。
func runSubscribeOnce(sdkClient ChainSdkInterface, cc *config.ContractConf,
	chainConfName, contractConfName, chainType string, logger logx.Logger,
	chainConfigID, contractConfigID uint, flagKey string) {

	// 检查是否重复订阅
	val, ok := SubscribeFlag.Load(flagKey)
	if ok {
		if b, _ := val.(bool); b {
			logger.Infof("[chain: %s] [contract: %s] already subscribed (key=%s)", chainConfName, contractConfName, flagKey)
			return
		}
	}

	// 标记为已订阅
	SubscribeFlag.Store(flagKey, true)

	// 退出路径保证清理 SubscribeFlag，使得下一次 subscribeRescheduleInterval 轮询可触发重订阅
	defer SubscribeFlag.Delete(flagKey)

	// 使用局部 subErr，不与外层共享，避免并发写入竞争
	subErr := sdkClient.SubscribeContractEvent(
		*cc, chainConfName, contractConfName, chainType, chainConfigID, contractConfigID)
	if subErr != nil {
		logger.Errorf("failed to subscribe chain %s contract %s event (key=%s): %v",
			chainConfName, contractConfName, flagKey, subErr)
		return
	}

	logger.Infof("[chain: %s] [contract: %s] subscribe goroutine returned normally (key=%s)",
		chainConfName, contractConfName, flagKey)
}

// scheduleDBOnce 执行一次 DB 配置路径的订阅扫描
// 从数据库查询所有 EnableSubscribe=true 的合约配置，为每个合约启动订阅
func scheduleDBOnce(ctx context.Context, tenantMgr *TenantSDKManager, repo store.Repository,
	logger logx.Logger, redisClient *commonEvent.RedisClient, factory ChainClientFactory, logConf logx.LogConf) {

	// 查询所有启用订阅的合约配置
	contracts, err := repo.ListAllEnabledSubscribeContracts(ctx)
	if err != nil {
		logger.Errorf("scheduleDBOnce: failed to list enabled subscribe contracts: %v", err)
		return
	}

	for _, contract := range contracts {
		// 跳过链未启用的合约
		if !contract.ChainConfig.Enable {
			continue
		}

		chainConfig := contract.ChainConfig
		chainType := strings.ToLower(chainConfig.ChainType)

		// 获取或创建 SDK 客户端
		sdkClient, clientErr := tenantMgr.GetTenantSDKClient(ctx, chainConfig.TenantID, chainConfig.ChainName)
		if clientErr != nil {
			logger.Errorf("scheduleDBOnce: failed to get SDK client for chain %s (ID=%d): %v",
				chainConfig.ChainName, chainConfig.ID, clientErr)
			continue
		}

		// 构建合约配置
		cc := &config.ContractConf{
			ContractName:    contract.ContractName,
			ContractAddr:    contract.ContractAddr,
			Abi:             contract.AbiJSON,
			EnableSubscribe: true,
		}

		// 解析额外配置
		if contract.ExtraConf != "" {
			_ = json.Unmarshal([]byte(contract.ExtraConf), cc)
			cc.ContractName = contract.ContractName
			cc.ContractAddr = contract.ContractAddr
			if contract.AbiJSON != "" {
				cc.Abi = contract.AbiJSON
			}
			cc.EnableSubscribe = true
		}

		// 使用 DB ID 作为 SubscribeFlag key，与配置文件路径隔离
		flagKey := subscribeKeyByID(chainConfig.ID, contract.ID)

		// 检查是否已订阅
		if val, ok := SubscribeFlag.Load(flagKey); ok {
			if b, _ := val.(bool); b {
				continue
			}
		}

		go runSubscribeOnce(sdkClient, cc, chainConfig.ChainName, contract.ContractName,
			chainType, logger, chainConfig.ID, contract.ID, flagKey)
	}
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
