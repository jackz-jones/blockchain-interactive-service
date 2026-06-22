package sdk

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/jackz-jones/blockchain-interactive-service/internal/store"

	commonEvent "github.com/jackz-jones/common/event"
	"github.com/zeromicro/go-zero/core/logx"
)

// ChainClientFactory 链客户端工厂函数类型
// 根据链名称、链类型和配置创建 ChainSdkInterface 实例
// 引入此抽象层使 sdk 包不直接依赖 plugin 包，避免循环依赖
type ChainClientFactory func(ctx context.Context, chainName, chainType string,
	chainConf *ChainConf, logConf logx.LogConf, redisClient *commonEvent.RedisClient) (ChainSdkInterface, error)

var (
	// SubscribeFlag 全局订阅标记: flagKey -> true
	SubscribeFlag = sync.Map{}
)

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

// SubscribeKeyByID 生成基于 DB ID 的 SubscribeFlag key
func SubscribeKeyByID(chainConfigID, contractConfigID uint) string {
	return fmt.Sprintf("db:%d-%d", chainConfigID, contractConfigID)
}

// StartSubscribe 启动订阅
// 行为说明：
//  1. 主 goroutine 每 subscribeRescheduleInterval 轮询一次数据库中所有启用订阅的合约。
//  2. 对已启用订阅但 SubscribeFlag 为 false/不存在的合约，启动订阅 goroutine。
//  3. 订阅 goroutine 内部使用局部 err 变量，消除与外层变量的数据竞争。
//  4. 订阅 goroutine 退出时通过 defer 清理 SubscribeFlag，使下一轮可重订阅。
//  5. 主 goroutine 监听 ctx.Done，收到退出信号后立即返回。
func StartSubscribe(ctx context.Context, logger logx.Logger,
	tenantMgr *TenantSDKManager, repo store.Repository) {
	if repo == nil || tenantMgr == nil {
		logger.Error("StartSubscribe: repo or tenantMgr is nil, skip")
		return
	}

	go func() {
		ticker := time.NewTicker(subscribeRescheduleInterval)
		defer ticker.Stop()
		for {
			// DB 配置路径订阅
			scheduleDBOnce(ctx, tenantMgr, repo, logger)

			// 每隔 subscribeRescheduleInterval 重新检查，或被 ctx 中断退出
			select {
			case <-ctx.Done():
				logger.Info("StartSubscribe root ctx done, exiting reschedule loop")
				return
			case <-ticker.C:
			}
		}
	}()
}

// runSubscribeOnce 在独立 goroutine 中执行一次订阅。
// - 使用局部 subErr 变量，不与外层共享。
// - defer 中清理 SubscribeFlag，使得下一次轮询可重订阅。
// - flagKey 由调用方传入。
func runSubscribeOnce(sdkClient ChainSdkInterface, cc *ContractConf,
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
	logger logx.Logger) {

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
		cc := &ContractConf{
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

		// 使用 DB ID 作为 SubscribeFlag key
		flagKey := SubscribeKeyByID(chainConfig.ID, contract.ID)

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
