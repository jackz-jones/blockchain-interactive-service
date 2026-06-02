package sdk

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"

	pb "github.com/jackz-jones/blockchain-interactive-service/pb"

	"chainmaker.org/chainmaker/pb-go/v2/common"
	chainmakersdk "chainmaker.org/chainmaker/sdk-go/v2"
	commonChain "github.com/jackz-jones/common/chain"
	commonEvent "github.com/jackz-jones/common/event"
	"github.com/zeromicro/go-zero/core/logx"
)

// ChainMakerClient 定义了长安链客户端对象
type ChainMakerClient struct {
	// ctx 由外部传入，用作客户端根上下文（Stop 后会被 cancel）
	ctx    context.Context
	cancel context.CancelFunc

	// wg 用于等待订阅 goroutine 退出
	wg sync.WaitGroup

	// 合约配置，配置名称--》合约名称
	contractConfigs map[string]string
	chainClient     *chainmakersdk.ChainClient
	logx.Logger

	// redis 客户端
	redisClient *commonEvent.RedisClient
}

// NewChainMakerClient 创建一个长安链客户端对象（使用结构化配置参数）
// 复用 common/chain.CreateSDKClient 方法创建底层 SDK 客户端
func NewChainMakerClient(ctx context.Context, chainConfName string, conf ChainMakerConf,
	contractConfs map[string]*ContractConf, logConf logx.LogConf,
	redisClient *commonEvent.RedisClient) (*ChainMakerClient, error) {

	// 必须配置签名私钥（链账户）
	if conf.SignKey == "" {
		return nil, fmt.Errorf("sign_key is required: chainmaker client needs a signing key to operate")
	}

	// 证书模式下必须配置签名证书
	if conf.AuthType == "permissionedwithcert" && conf.SignCert == "" {
		return nil, fmt.Errorf("sign_cert is required when auth_type is 'permissionedwithcert'")
	}

	// 构建 common/chain.NodeConf 列表
	nodeConfs := make([]commonChain.NodeConf, 0, len(conf.Nodes))
	for _, n := range conf.Nodes {
		nodeConfs = append(nodeConfs, commonChain.NodeConf{
			Url:         n.NodeAddr,
			EnableTls:   n.EnableTls,
			TlsHostName: n.TlsHostName,
			CaCert:      n.CaCert,
		})
	}

	// 确定链模式
	chainMode := conf.AuthType

	// 日志路径
	logPath := logConf.Path + fmt.Sprintf("/sdk-%s.log", chainConfName)

	// 复用 common/chain.CreateSDKClient 创建底层 SDK 客户端
	client, err := commonChain.CreateSDKClient(
		nodeConfs,
		conf.ChainId,
		chainMode,
		conf.SignKey,
		conf.SignCert,
		conf.HashType,
		conf.OrgId,
		conf.UserTlsCert,
		conf.UserTlsKey,
		conf.UserEncCert,
		conf.UserEncKey,
		logPath,
		conf.ProxyUrl,
		logConf.Level,
		logConf.KeepDays,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create chainmaker sdk client: %v", err)
	}

	// 解析配置的合约名称
	contracts := make(map[string]string)
	for configName, c := range contractConfs {
		contracts[configName] = c.ContractName
	}

	// 基于父 ctx 派生可取消子 ctx，用于订阅 goroutine 的生命周期控制
	childCtx, cancel := context.WithCancel(ctx)

	return &ChainMakerClient{
		ctx:             childCtx,
		cancel:          cancel,
		chainClient:     client,
		contractConfigs: contracts,
		Logger:          logx.WithContext(childCtx),
		redisClient:     redisClient,
	}, nil
}

// GetTxByTxId 获取交易
func (c *ChainMakerClient) GetTxByTxId(txId string) (string, bool, error) {
	txInfo, err := c.chainClient.GetTxByTxId(txId)
	if err != nil {
		return "", false, fmt.Errorf("failed to GetTxByTxId: %v", err)
	}

	// 统一成 common.TxResponse 结构
	txResp := &common.TxResponse{
		Code:           txInfo.Transaction.Result.Code,
		Message:        txInfo.Transaction.Result.Message,
		ContractResult: txInfo.Transaction.Result.ContractResult,
		TxTimestamp:    txInfo.Transaction.Payload.Timestamp,
		TxBlockHeight:  txInfo.BlockHeight,
		TxId:           txId,
	}

	txBytes, err := json.Marshal(txResp)
	if err != nil {
		return "", false, fmt.Errorf("failed to json marshal tx response: %v", err)
	}

	return string(txBytes), txInfo.BlockHeight != 0, nil

}

// CallContract 调用合约
func (c *ChainMakerClient) CallContract(methodType pb.MethodType, contractConfigName, method string,
	args []*pb.KeyValuePair, txTimeout int64, withSyncResult bool) (string, string, error) {
	var (
		txResp *common.TxResponse
		err    error
	)

	// 解析配置的合约名称
	contractName, ok := c.contractConfigs[contractConfigName]
	if !ok {
		return "", "", fmt.Errorf("unknown contract config name %s", contractConfigName)
	}

	// 解析长安链的参数
	kvs := make([]*common.KeyValuePair, len(args))
	for i, a := range args {

		// 转换成长安链的 kv
		kvs[i] = &common.KeyValuePair{
			Key:   a.Key,
			Value: a.Value,
		}
	}

	switch methodType {

	// 写链
	case pb.MethodType_Invoke:
		txResp, err = c.chainClient.InvokeContract(contractName, method, "", kvs, txTimeout, withSyncResult)
		if err != nil {
			return "", "", fmt.Errorf("failed to InvokeContract: %v", err)
		}

	// 读链
	case pb.MethodType_Query:
		txResp, err = c.chainClient.QueryContract(contractName, method, kvs, txTimeout)
		if err != nil {
			return "", "", fmt.Errorf("failed to QueryContract: %v", err)
		}

	// 不支持的合约调用类型
	default:
		return "", "", fmt.Errorf("unsupported method type %d", methodType)
	}

	txBytes, err := json.Marshal(txResp)
	if err != nil {
		return "", "", fmt.Errorf("failed to json marshal tx response: %v", err)
	}

	return txResp.TxId, string(txBytes), nil
}

// Stop 停止客户端
func (c *ChainMakerClient) Stop() error {
	if c.cancel != nil {
		c.cancel()
	}

	c.wg.Wait()
	if c.chainClient != nil {
		return c.chainClient.Stop()
	}

	return nil
}

// SubscribeContractEvent 订阅合约事件
func (c *ChainMakerClient) SubscribeContractEvent(contractConf ContractConf, chainConfName, contractConfName,
	chainType string, chainConfigID, contractConfigID uint) error {

	// 日志通用信息
	logFields := BuildSubscribeLogFields(map[string]interface{}{
		"chainConfName":     chainConfName,
		"contractConfName":  contractConfName,
		"contractName":      contractConf.ContractName,
		"deployBlockHeight": contractConf.DeployBlockHeight,
		"module":            "subscribeChainmaker",
	})

	// 检查合约名称不为空
	if contractConf.ContractName == "" {
		c.Logger.WithFields(logFields...).Error("chainmaker contract name empty")
		return errors.New("chainmaker contract name empty")
	}

	// 构建 Redis key：DB 路径使用 ID 格式，配置文件路径使用旧格式
	var blockHeightKey string
	if chainConfigID > 0 && contractConfigID > 0 {
		blockHeightKey = fmt.Sprintf("block_height:%d:%d", chainConfigID, contractConfigID)
	} else {
		blockHeightKey = strings.Join([]string{chainType, chainConfName, contractConfName}, "#")
	}

	// 获取最新区块高度
	height, err := c.redisClient.GetLatestBlockHeight(c.ctx, blockHeightKey)
	if err != nil {
		c.Logger.WithFields(logFields...).Errorf("failed to GetLatestBlockHeight: %v", err)
		return fmt.Errorf("failed to GetLatestBlockHeight: %v", err)
	}

	// 如果 redis 中区块高度为0，则从合约部署高度开始订阅
	if height == 0 {
		height = contractConf.DeployBlockHeight
	}

	// 日志增加 startHeight
	logFields = append(logFields, logx.Field("startHeight", height))
	c.Logger.WithFields(logFields...).Infof("success to GetLatestBlockHeight for chainmaker chain %s contract %s ",
		chainConfName, contractConfName)

	// 发送订阅交易，区间为 [height, latest]
	subChan, err := c.chainClient.SubscribeContractEvent(c.ctx, int64(height), -1, contractConf.ContractName, "")
	if err != nil {
		c.Logger.WithFields(logFields...).Errorf("failed to SubscribeContractEvent: %v", err)
		return fmt.Errorf("failed to SubscribeContractEvent: %v", err)
	}

	// 等待订阅协程
	c.wg.Add(1)
	defer c.wg.Done()

	c.Logger.WithFields(logFields...).Infof("success to SubscribeContractEvent")
	for {
		select {

		// 接收到合约事件
		case eventInfo, ok := <-subChan:

			// 订阅事件通道关闭
			if !ok {
				return fmt.Errorf("chan closed")
			}

			// 接收到nil事件
			if eventInfo == nil {
				c.Logger.WithFields(logFields...).Error("received nil eventInfo")
				continue
			}

			// 转化合约事件结构
			contractEventInfo, ok := eventInfo.(*common.ContractEventInfo)
			if !ok {
				c.Logger.WithFields(logFields...).Error("failed to convert to common.ContractEventInfo")
				continue
			}

			if len(contractEventInfo.EventData) == 0 {
				c.Logger.WithFields(logFields...).Infof("block %d has no eventInfo data", contractEventInfo.BlockHeight)
				continue
			}

			// 发布合约事件到redis stream
			c.Logger.WithFields(logFields...).Infof("received chainmaker contract eventInfo[txid: %s, height: %d]",
				contractEventInfo.TxId, contractEventInfo.BlockHeight)
			if err = c.redisClient.PublishCrossChainEventToStream(c.ctx, contractEventInfo,
				chainConfigID, contractConfigID, contractEventInfo.Topic); err != nil {
				c.Logger.WithFields(logFields...).Errorf("failed to publish eventInfo to redis stream: %v", err)
				return err
			}
			c.Logger.WithFields(logFields...).Infof("publish new chainmaker contract eventInfo to stream: %v", contractEventInfo)

			// 更新处理高度，有可能同一个高度有多个事件
			if contractEventInfo.BlockHeight > height {
				err = c.redisClient.SetLatestBlockHeight(c.ctx, blockHeightKey, contractEventInfo.BlockHeight)
				if err != nil {
					c.Logger.WithFields(logFields...).Errorf("failed to SetLatestBlockHeight: %v", err)
					return err
				}

				c.Logger.WithFields(logFields...).Infof("set new chainmaker block height[%d]", contractEventInfo.BlockHeight)
				height = contractEventInfo.BlockHeight
			}

		// 接收到退出信号
		case <-c.ctx.Done():
			c.Logger.WithFields(logFields...).Errorf("ctx done")
			return nil
		}
	}
}
