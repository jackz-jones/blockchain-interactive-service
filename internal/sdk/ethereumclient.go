package sdk

import (
	"context"
	"crypto/ecdsa"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum"

	"github.com/jackz-jones/blockchain-interactive-service/internal/config"
	"github.com/jackz-jones/blockchain-interactive-service/internal/util"
	pb "github.com/jackz-jones/blockchain-interactive-service/pb"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/jackz-jones/common/chain"
	commonEvent "github.com/jackz-jones/common/event"
	"github.com/zeromicro/go-zero/core/logx"
)

// EthereumClient 定义了以太坊客户端对象
type EthereumClient struct {
	ctx         context.Context
	chainId     *big.Int
	gasLimit    uint64
	privateKey  *ecdsa.PrivateKey
	fromAddress common.Address

	// 合约配置，配置名称--》合约信息
	contractConfigs map[string]*config.ContractConf

	// http 和 websocket 连接，前者用于发交易和查询，后者用于订阅事件
	httpClient ethclient.Client
	wsClient   ethclient.Client
	logx.Logger

	// 合约事件处理器集合
	ethEventHandlers map[string]*commonEvent.EthEventHandler
	redisClient      *commonEvent.RedisClient
}

// NewEthereumClient 创建一个 EthereumClient 对象
func NewEthereumClient(ctx context.Context, ethConf config.EthConf, contractConfs map[string]*config.ContractConf,
	redisClient *commonEvent.RedisClient) (*EthereumClient, error) {

	// 建立 http 连接
	httpClient, err := ethclient.DialContext(ctx, ethConf.HttpUrl)
	if err != nil {
		return nil, fmt.Errorf("failed to connect ethereum http port: %v", err)
	}

	// 建立 websocket 连接
	wsClient, err := ethclient.DialContext(ctx, ethConf.WebsocketUrl)
	if err != nil {
		return nil, fmt.Errorf("failed to connect ethereum websocket port: %v", err)
	}

	// 获取发送者的地址
	privateKey, err := crypto.HexToECDSA(ethConf.PrivateKey)
	if err != nil {
		return nil, fmt.Errorf("failed to generate private key: %v", err)
	}

	// 获取私钥对应的地址
	publicKey := privateKey.Public()
	publicKeyECDSA, ok := publicKey.(*ecdsa.PublicKey)
	if !ok {
		return nil, fmt.Errorf("failed to cast public key to ECDSA")
	}

	// 初始化合约事件处理器
	ethEventHandlers := make(map[string]*commonEvent.EthEventHandler)
	for contractConfName, c := range contractConfs {

		// 读取 abi 文件
		abiJson, err2 := util.ReadAbiJsonFile(c.Abi)
		if err2 != nil {
			return nil, fmt.Errorf("failed to ReadAbiJsonFile for %s: %v", contractConfName, err2)
		}

		// 初始化合约事件处理器
		eventHandler, err2 := commonEvent.NewEthEventHandler(abiJson)
		if err2 != nil {
			return nil, fmt.Errorf("failed to NewEthEventHandler for %s: %v", contractConfName, err2)
		}

		ethEventHandlers[contractConfName] = eventHandler
	}

	return &EthereumClient{
		chainId:          big.NewInt(ethConf.ChainId),
		gasLimit:         ethConf.GasLimit,
		privateKey:       privateKey,
		fromAddress:      crypto.PubkeyToAddress(*publicKeyECDSA),
		contractConfigs:  contractConfs,
		ctx:              ctx,
		httpClient:       *httpClient,
		wsClient:         *wsClient,
		Logger:           logx.WithContext(ctx),
		ethEventHandlers: ethEventHandlers,
		redisClient:      redisClient,
	}, nil
}

// GetTxByTxId 根据 txId 查询交易
func (c *EthereumClient) GetTxByTxId(txId string) (string, bool, error) {

	// 以太坊交易简化结构
	ethTx := EthTx{
		TxId: txId,
	}

	// 查询交易
	tx, pending, err := c.httpClient.TransactionByHash(c.ctx, common.HexToHash(txId))
	if err != nil {
		return "", false, fmt.Errorf("failed to req TransactionByHash: %v", err)
	}

	// 如果交易未上链，返回 pending 状态
	if pending {
		return ethTx.String(), true, nil
	}

	// 获取交易发起者
	fromm, err := types.Sender(types.NewEIP155Signer(c.chainId), tx)
	if err != nil {
		return "", false, fmt.Errorf("failed to get tx sender: %v", err)
	}

	ethTx.From = fromm.Hex()
	ethTx.To = tx.To().Hex()

	// 如果交易已上链，查询 receipt
	receipt, err := c.httpClient.TransactionReceipt(c.ctx, common.HexToHash(txId))
	if err != nil {
		return "", false, fmt.Errorf("failed to req TransactionReceipt: %v", err)
	}

	ethTx.Status = receipt.Status

	// 如果交易失败，则返回失败原因
	if receipt.Status == 0 {

		// 模拟 eth_call 执行，获取失败原因
		_, err = c.httpClient.CallContract(c.ctx, ethereum.CallMsg{
			To:   tx.To(),
			Data: tx.Data(),
		}, receipt.BlockNumber)
		ethTx.Msg = err.Error()
		return ethTx.String(), false, nil
	}

	return ethTx.String(), false, nil
}

// SendTransaction 调用 eth_sendTransaction 方法，交易执行状态会上链，一般用于写数据类型调用
func (c *EthereumClient) SendTransaction(methodType pb.MethodType, contractConfigName, method string,
	kvs []*pb.KeyValuePair, txTimeout int64, withSyncResult bool) (string, string, error) {
	var (

		// 以太坊的合约调用返回不是标准的 Transaction 结构
		txResp interface{}
		err    error
		txId   string
	)

	// 获取合约地址和 abi 字符串
	contractConf, ok := c.contractConfigs[contractConfigName]
	if !ok {
		return "", "", fmt.Errorf("unknown contract config name %s", contractConfigName)
	}

	contractAddr := contractConf.ContractAddr

	// 读取 abi 字符串
	abiStr, err := util.ReadAbiJsonFile(contractConf.Abi)
	if err != nil {
		return "", "", fmt.Errorf("failed to ReadAbiJsonFile: %v", err)
	}

	// solidity 合约也支持方法传参结构体类型，这里需要将统一请求的 kvs 翻译成以太坊的 abi 参数结构
	args, err := c.CreateArgs(method, kvs)
	if err != nil {
		return "", "", fmt.Errorf("failed to CreateArgs: %v", err)
	}

	switch methodType {

	// 写链
	case pb.MethodType_Invoke:
		txId, err = c.InvokeContract(contractAddr, abiStr, method, args...)
		if err != nil {
			return "", "", fmt.Errorf("failed to InvokeContract: %v", err)
		}

		// 如果异步调用，直接返回 txId
		txResp = map[string]string{"txId": txId}
		if withSyncResult {
			ticker := time.NewTicker(time.Second)
			defer ticker.Stop()

			// 同步调用，需要轮训交易结果
			for {
				select {

				// 每隔一秒检查交易执行结果
				case <-ticker.C:
					txReceipt, err2 := c.httpClient.TransactionReceipt(c.ctx, common.HexToHash(txId))
					if err2 != nil {
						c.Logger.Errorf("failed to TransactionReceipt: %v", err2)
						continue
					}

					// 成功查到返回结果
					txResp = txReceipt

				// 查询超时返回
				case <-time.After(time.Duration(txTimeout) * time.Second):
					txBytes, err2 := json.Marshal(txResp)
					if err2 != nil {
						return "", "", fmt.Errorf("failed to json marshal tx response: %v", err2)
					}

					// 查询 receipt 超时，返回错误同时也应该返回 txId
					return txId, string(txBytes), fmt.Errorf("sync to get tx receipt timeout, maybe try it later")
				}
			}
		}

	// 读链
	case pb.MethodType_Query:
		txResp, err = c.QueryContract(contractAddr, abiStr, method, args...)
		if err != nil {
			return "", "", fmt.Errorf("failed to QueryContract: %v", err)
		}

	// 不支持的合约调用类型
	default:
		return "", "", fmt.Errorf("unsupported method type %d", methodType)
	}

	// 统一序列化成 json 字符串
	txBytes, err := json.Marshal(txResp)
	if err != nil {
		return txId, "", fmt.Errorf("failed to json marshal tx response: %v", err)
	}

	// query 类产生的 txId 不上链，暂不返回
	return txId, string(txBytes), nil
}

/*
* @Description: InvokeContract 调用 eth_sendTransaction 方法，交易执行状态会上链，一般用于写数据类型调用
* @receiver c

* @param contractAddr 合约地址
* @param abiStr 合约 abi 字符串
* @param method 合约方法名
* @param args 合约参数

* @return string 交易哈希
* @return error 错误信息
 */

// InvokeContract 调用 eth_sendTransaction 方法，交易执行状态会上链，一般用于写数据类型调用
func (c *EthereumClient) InvokeContract(contractAddr, abiStr, method string, args ...interface{}) (string, error) {

	// to 为合约地址
	toAddress := common.HexToAddress(contractAddr)

	// 实时获取 gas 价格
	gasPrice, err := c.httpClient.SuggestGasPrice(c.ctx)
	if err != nil {
		return "", fmt.Errorf("failed to SuggestGasPrice: %v", err)
	}

	// 获取 from 账户的 nonce
	nonce, err := c.httpClient.PendingNonceAt(c.ctx, c.fromAddress)
	if err != nil {
		return "", fmt.Errorf("failed to PendingNonceAt: %v", err)
	}

	// 构造 input data
	data, err := c.CreateInputData(abiStr, method, args...)
	if err != nil {
		return "", fmt.Errorf("failed to CreateInputData in InvokeContract: %v", err)
	}

	// 构造交易
	tx := types.NewTransaction(nonce, toAddress, nil, c.gasLimit, gasPrice, data)

	// 签名交易
	signer := types.NewEIP155Signer(c.chainId)
	signedTx, err := types.SignTx(tx, signer, c.privateKey)
	if err != nil {
		return "", fmt.Errorf("failed to SignTx: %v", err)
	}

	// 发送交易
	err = c.httpClient.SendTransaction(c.ctx, signedTx)
	if err != nil {
		return "", fmt.Errorf("failed to SendTransaction: %v", err)
	}

	return signedTx.Hash().Hex(), nil
}

/*
* @Description: QueryContract 调用 eth_call 方法，只是在 vm 中执行返回结果，但是状态不会上链，一般用于查询类调用
* @receiver c

* @param contractAddr 合约地址
* @param abiStr 合约 abi 字符串
* @param method 合约方法名
* @param args 合约参数

* @return interface{} 合约执行结果
* @return error 错误信息
 */

// QueryContract 调用 eth_call 方法，只是在 vm 中执行返回结果，但是状态不会上链，一般用于查询类调用
func (c *EthereumClient) QueryContract(contractAddr, abiStr, method string, args ...interface{}) (interface{}, error) {

	// 合约ABI
	contractABI, err := abi.JSON(strings.NewReader(abiStr))
	if err != nil {
		return nil, fmt.Errorf("failed to abi.JSON in QueryContract: %v", err)
	}

	// 构造 input data
	data, err := c.CreateInputData(abiStr, method, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to CreateInputData in QueryContract: %v", err)
	}

	// 构造调用请求
	contract := common.HexToAddress(contractAddr)
	msg := ethereum.CallMsg{
		From: c.fromAddress,
		To:   &contract,
		Gas:  10000000,
		Data: data,
	}

	// 调用合约函数
	result, err := c.httpClient.CallContract(c.ctx, msg, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to CallContract: %v", err)
	}

	// 解析返回值
	resultValue, err := contractABI.Unpack(method, result)
	if err != nil {
		return nil, fmt.Errorf("failed to contractABI.Unpack: %v", err)
	}

	return resultValue, nil
}

// CreateInputData 构造以太坊合约调用的 input data
func (c *EthereumClient) CreateInputData(abiStr, method string, args ...interface{}) ([]byte, error) {

	// 解析ABI
	contractABI, err := abi.JSON(strings.NewReader(abiStr))
	if err != nil {
		return nil, fmt.Errorf("failed to abi.JSON in CreateInputData: %v", err)
	}

	// 构造 input data 结构
	data, err := contractABI.Pack(method, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to contractABI.Pack: %v", err)
	}

	return data, nil
}

// Stop 停止客户端
func (c *EthereumClient) Stop() error {
	c.httpClient.Close()
	c.wsClient.Close()
	return nil
}

// SubscribeContractEvent 订阅合约事件
func (c *EthereumClient) SubscribeContractEvent(contractConf config.ContractConf, chainConfName,
	contractConfName, chainType, contractType string) error {

	// 日志通用信息
	fields := map[string]interface{}{
		"chainConfName":    chainConfName,
		"contractConfName": contractConfName,
		"contractAddr":     contractConf.ContractAddr,
		"contractAbi":      contractConf.Abi,
		"contractType":     contractConf.ContractType,
		"module":           "subscribeEth",
	}
	logFields := make([]logx.LogField, 0)
	for k, v := range fields {
		logFields = append(logFields, logx.Field(k, v))
	}

	// 检查合约地址不为空
	if contractConf.ContractAddr == "" {
		c.Logger.WithFields(logFields...).Error("eth contract address empty")
		return errors.New("eth contract address empty")
	}

	// 获取最新区块高度
	height, err := c.redisClient.GetLatestBlockHeight(c.ctx, strings.Join([]string{chainType, chainConfName, contractType,
		contractConfName}, "#"))
	if err != nil {
		c.Logger.WithFields(logFields...).Errorf("failed to GetLatestBlockHeight: %v", err)
		return fmt.Errorf("failed to GetLatestBlockHeight: %v", err)
	}

	// 如果 redis 中区块高度为0，则从合约部署高度开始订阅
	if height == 0 {
		height = contractConf.DeployBlockHeight
	} else {

		// redis 中存储的是处理过的最新高度，新的处理需要+1
		height++
	}

	logFields = append(logFields, logx.Field("startHeight", height))
	c.Logger.WithFields(logFields...).Infof("success to GetLatestBlockHeight %d for eth chain %s contract %s ", height,
		chainConfName, contractConfName)

	// 实时订阅合约事件
	err = c.GetHistoryEvent(contractConf.ContractAddr, chainConfName, contractConfName, chainType, contractType, height,
		contractConf.GetHistoryEventHeightWindow, contractConf.GetHistoryEventInterval, logFields)
	if err != nil {
		c.Logger.WithFields(logFields...).Errorf("failed to GetHistoryEvent: %v", err)
		return fmt.Errorf("failed to GetHistoryEvent: %v", err)
	}

	return nil
}

// GetHistoryEvent 获取历史合约事件，实时事件可能会因为链分叉重组而移除，实时事件不是最终的，历史的比较准确
func (c *EthereumClient) GetHistoryEvent(contractAddr, chainConfName, contractConfName, chainType, contractType string,
	startHeight, window, interval uint64, logFields []logx.LogField) error {

	// 定时去获取一次历史合约事件，以太坊 2.0 是 12s 出一个块

	ticker := time.NewTicker(time.Millisecond * time.Duration(interval))
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:

			// 获取以太坊链上当前最新块高
			latestHeight, err := c.httpClient.BlockNumber(c.ctx)
			if err != nil {
				c.Logger.WithFields(logFields...).Errorf("failed to get eth latest block height: %v", err)
				break
			}

			// 如果 startHeight 大于当前最新块高，则等待
			if startHeight > latestHeight {
				c.Logger.WithFields(logFields...).Infof("startHeight %d is bigger than eth chain latestHeight %d,"+
					" wait for new block mined", startHeight, latestHeight)
				continue
			}

			// 如果 endHeight  大于当前最新块高，则继续查询 [startHeight, latestHeight] 范围
			endHeight := startHeight + window
			if endHeight > latestHeight {
				endHeight = latestHeight
			}

			// 日志增加查询区块范围
			logFields = append(logFields, logx.Field("startHeight", startHeight), logx.Field("endHeight", endHeight))
			c.Logger.WithFields(logFields...).Infof("start to get eth contract events")

			// 采用窗口查询方式，避免查询时间过长

			fromBlock := big.NewInt(int64(startHeight))

			toBlock := big.NewInt(int64(endHeight))

			// 区块范围是闭区间 [FromBlock, ToBlock]，toBlock传空表示到最新区块
			query := ethereum.FilterQuery{
				Addresses: []common.Address{common.HexToAddress(contractAddr)},
				FromBlock: fromBlock,
				ToBlock:   toBlock,
			}

			// 查询合约事件
			logs, err := c.wsClient.FilterLogs(c.ctx, query)
			if err != nil {
				c.Logger.WithFields(logFields...).Errorf("failed to FilterLogs: %v", err)
				continue
			}

			// 如果没有事件返回，更新处理高度，继续查询
			if len(logs) == 0 {

				// 即使窗口中没有事件，也更新处理高度
				err = c.redisClient.SetLatestBlockHeight(c.ctx, strings.Join([]string{chainType, chainConfName, contractType,
					contractConfName}, "#"), endHeight)
				if err != nil {
					c.Logger.WithFields(logFields...).Errorf("failed to SetLatestBlockHeight: %v", err)
					break
				}

				c.Logger.WithFields(logFields...).Infof("no logs, also set new eth block height[%d]", endHeight)
				startHeight = endHeight + 1
				continue
			}

			c.Logger.WithFields(logFields...).Infof("received %d eth contract events", len(logs))

			// 处理事件
			for _, vLog := range logs {
				c.Logger.WithFields(logFields...).Infof("received eth contract event[height: %d]: %#v", vLog.BlockNumber, vLog)

				// 解析事件名称
				eventName := ""
				eventName, err = c.ethEventHandlers[contractConfName].EventName(vLog)
				if err != nil {
					c.Logger.WithFields(logFields...).Errorf("failed to get eth event name from vLog: %#v", vLog)
					break
				}

				// 推送整个 log 结构到 redis，通过 log 里面的 topic 识别具体的事件类型，才能正确解析 log 里的事件数据 data
				if err = c.redisClient.PublishTradeGuardEventToStream(c.ctx, vLog, chainType, chainConfName, contractType,
					contractConfName, eventName); err != nil {
					c.Logger.WithFields(logFields...).Errorf("failed to publish eth event to redis stream: %v", err)
					break
				}
				c.Logger.WithFields(logFields...).Infof("publish eth contract event to redis stream: %#v", vLog)

				// 更新处理高度，以及下一次 startHeight
				if vLog.BlockNumber >= startHeight {
					err = c.redisClient.SetLatestBlockHeight(c.ctx, strings.Join([]string{chainType, chainConfName, contractType,
						contractConfName}, "#"), vLog.BlockNumber)
					if err != nil {
						c.Logger.WithFields(logFields...).Errorf("failed to SetLatestBlockHeight: %v", err)
						break
					}

					c.Logger.WithFields(logFields...).Infof("set new eth block height[%d]", vLog.BlockNumber)
					startHeight = vLog.BlockNumber + 1
				}
			}

		// 接收到退出信号
		case <-c.ctx.Done():
			c.Logger.WithFields(logFields...).Errorf("ctx done")
			return nil
		}
	}
}

// RealTimeEvent 实时订阅合约事件，只会接受此时开始发生的事件，过去的历史事件不会返回
func (c *EthereumClient) RealTimeEvent(contractAddr, chainConfName, contractConfName, chainType, contractType string,
	height uint64, logFields []logx.LogField) error {

	// 过滤指定合约的事件
	query := ethereum.FilterQuery{
		Addresses: []common.Address{common.HexToAddress(contractAddr)},
	}

	// 创建通道以接收事件
	logs := make(chan types.Log)

	// 实时订阅合约事件，只会接受此时开始发生的事件，过去的历史事件不会返回
	sub, err := c.wsClient.SubscribeFilterLogs(context.Background(), query, logs)
	if err != nil {
		c.Logger.WithFields(logFields...).Errorf("failed to SubscribeFilterLogs: %v", err)
		return fmt.Errorf("failed to SubscribeFilterLogs: %v", err)
	}

	// 处理事件
	for {
		select {
		case err = <-sub.Err():
			c.Logger.WithFields(logFields...).Errorf("eth contract subscribe error: %v", err)
			return fmt.Errorf("eth contract subscribe error: %v", err)

		case vLog := <-logs:
			c.Logger.WithFields(logFields...).Infof("received eth contract event[height: %d]: %#v", vLog.BlockNumber, vLog)

			// 解析事件名称
			eventName := ""
			eventName, err = c.ethEventHandlers[contractConfName].EventName(vLog)
			if err != nil {
				c.Logger.WithFields(logFields...).Errorf("failed to get eth event name from vLog: %#v", vLog)
				break
			}

			// 推送整个 log 结构到 redis，通过 log 里面的 topic 识别具体的事件类型，才能正确解析 log 里的事件数据 data
			if err = c.redisClient.PublishTradeGuardEventToStream(c.ctx, vLog, chainType, chainConfName, contractType,
				contractConfName, eventName); err != nil {
				c.Logger.WithFields(logFields...).Errorf("failed to publish event to redis stream: %v", err)
				return err
			}
			c.Logger.WithFields(logFields...).Infof("publish eth contract event to stream: %#v", vLog)

			// 更新最新区块高度
			if vLog.BlockNumber > height {
				err = c.redisClient.SetLatestBlockHeight(c.ctx, strings.Join([]string{chainType, chainConfName, contractType,
					contractConfName}, "#"), vLog.BlockNumber)
				if err != nil {
					c.Logger.WithFields(logFields...).Errorf("failed to SetLatestBlockHeight: %v", err)
					return err
				}

				c.Logger.WithFields(logFields...).Infof("set eth block height[%d]", vLog.BlockNumber)
				height = vLog.BlockNumber
			}

		// 接收到退出信号
		case <-c.ctx.Done():
			c.Logger.WithFields(logFields...).Errorf("ctx done")
			sub.Unsubscribe()
			return nil
		}
	}
}

func (c *EthereumClient) CreateArgs(method string, kvs []*pb.KeyValuePair) ([]interface{}, error) {
	params := make(map[string][]byte, 0)
	for _, kv := range kvs {
		params[kv.Key] = kv.Value
	}

	return chain.CreateArgs(method, params)
}
