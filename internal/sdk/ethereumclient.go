package sdk

import (
	"context"
	"crypto/ecdsa"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"strings"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum"

	"github.com/jackz-jones/blockchain-interactive-service/internal/code"
	"github.com/jackz-jones/blockchain-interactive-service/internal/util"
	pb "github.com/jackz-jones/blockchain-interactive-service/pb"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
	commonEvent "github.com/jackz-jones/common/event"
	"github.com/zeromicro/go-zero/core/logx"
)

// EthereumClient 定义了以太坊客户端对象
type EthereumClient struct {
	// ctx 由外部传入，用作客户端根上下文（Stop 后会被 cancel）
	ctx    context.Context
	cancel context.CancelFunc

	// wg 用于等待订阅 goroutine 退出
	wg          sync.WaitGroup
	chainId     *big.Int
	privateKey  *ecdsa.PrivateKey
	fromAddress common.Address

	// 合约配置，配置名称--》合约信息
	contractConfigs map[string]*ContractConf

	// http 和 websocket 连接，前者用于发交易和查询，后者用于订阅事件
	httpClient *ethclient.Client
	wsClient   *ethclient.Client
	logx.Logger

	// 合约事件处理器集合
	ethEventHandlers map[string]*commonEvent.EthEventHandler

	// abiJsonCache 合约 ABI JSON 字符串缓存（contractConfigName -> abiJson），
	// 避免 CallContract 每次调用都重复读取磁盘 ABI 文件。
	abiJsonCache map[string]string
	redisClient  *commonEvent.RedisClient
}

// NewEthereumClient 创建一个 EthereumClient 对象
func NewEthereumClient(ctx context.Context, ethConf EthConf, contractConfs map[string]*ContractConf,
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
	abiJsonCache := make(map[string]string)
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
		abiJsonCache[contractConfName] = abiJson
	}

	// 基于独立后台 context 派生可取消子 ctx，用于订阅 goroutine 的生命周期控制
	// 不使用请求传入的 ctx，避免请求结束后 context 被取消导致缓存的客户端不可用
	childCtx, cancel := context.WithCancel(context.Background())

	return &EthereumClient{
		chainId:          big.NewInt(ethConf.ChainId),
		privateKey:       privateKey,
		fromAddress:      crypto.PubkeyToAddress(*publicKeyECDSA),
		contractConfigs:  contractConfs,
		ctx:              childCtx,
		cancel:           cancel,
		httpClient:       httpClient,
		wsClient:         wsClient,
		Logger:           logx.WithContext(childCtx),
		ethEventHandlers: ethEventHandlers,
		abiJsonCache:     abiJsonCache,
		redisClient:      redisClient,
	}, nil
}

// GetTxByTxId 根据 txId 查询交易
func (c *EthereumClient) GetTxByTxId(txId string) (string, bool, error) {

	// 以太坊交易简化结构
	ethTx := EthTx{
		TxHash: txId,
	}

	// 查询交易
	tx, pending, err := c.httpClient.TransactionByHash(c.ctx, common.HexToHash(txId))
	if err != nil {
		return "", false, fmt.Errorf("failed to req TransactionByHash: %v", err)
	}

	// 如果交易是 pending 状态，说明还没确认打包，但至少交易已经发到节点了，直接返回交易信息和未确认状态
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

	// 交易成功，则返回打包所在区块、以及产生的事件信息
	ethTx.BlockHash = receipt.BlockHash.Hex()
	ethTx.BlockNumber = receipt.BlockNumber.Uint64()
	ethTx.TxIndex = receipt.TransactionIndex
	ethTx.Logs, err = json.Marshal(receipt.Logs)
	if err != nil {
		return "", false, fmt.Errorf("failed to marshal receipt.Logs: %v", err)
	}

	// 交易已经确认打包，非 pending
	return ethTx.String(), false, nil
}

// CallContract 调用合约
func (c *EthereumClient) CallContract(methodType pb.MethodType, contractConfigName, method string,
	kvs []*pb.KeyValuePair, txTimeout int64, withSyncResult bool, gasLimit int64) (string, string, error) {
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

	// 优先使用启动时缓存的 ABI，避免每次调用都重复读取磁盘
	abiStr, ok := c.abiJsonCache[contractConfigName]
	if !ok || abiStr == "" {
		// 极少数回退场景：contractConfigName 在启动后才动态增加，此时回退到文件读取
		var readErr error
		abiStr, readErr = util.ReadAbiJsonFile(contractConf.Abi)
		if readErr != nil {
			return "", "", fmt.Errorf("failed to ReadAbiJsonFile: %v", readErr)
		}
		c.Logger.Errorf("abi cache miss for [%s], fallback to file read", contractConfigName)
	}

	// solidity 合约也支持方法传参结构体类型，这里需要将统一请求的 kvs 翻译成以太坊的 abi 参数结构
	args, err := c.CreateArgs(method, kvs, abiStr)
	if err != nil {
		return "", "", fmt.Errorf("failed to CreateArgs: %v", err)
	}

	switch methodType {

	// 写链
	case pb.MethodType_Invoke:
		// gasLimit: 0=自动估算（EstimateGas + 20%余量），>0=使用指定值
		txId, err = c.InvokeContractWithGasLimit(contractAddr, abiStr, method, uint64(gasLimit), args...)
		if err != nil {
			return "", "", fmt.Errorf("failed to InvokeContract: %v", err)
		}

		// 如果异步调用，直接返回 txId
		txResp = map[string]string{"txId": txId}
		if withSyncResult {
			ticker := time.NewTicker(time.Second)
			defer ticker.Stop()

			// 使用独立的超时 timer，避免在 for-select 中重复创建 time.After
			timeoutTimer := time.NewTimer(time.Duration(txTimeout) * time.Second)
			defer timeoutTimer.Stop()

			// 同步调用，需要轮询交易结果
		syncLoop:
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
					break syncLoop

				// 查询超时返回
				case <-timeoutTimer.C:
					txBytes, err2 := json.Marshal(txResp)
					if err2 != nil {
						return "", "", fmt.Errorf("failed to json marshal tx response: %v", err2)
					}

					// 查询 receipt 超时，返回错误同时也应该返回 txId
					return txId, string(txBytes), fmt.Errorf("%s, txId: %s", code.ErrGetTxReceiptTimeoutMsg, txId)
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
// 默认使用自动估算 Gas（EstimateGas + 20%余量），如需指定 GasLimit 请使用 InvokeContractWithGasLimit
func (c *EthereumClient) InvokeContract(contractAddr, abiStr, method string, args ...interface{}) (string, error) {
	return c.InvokeContractWithGasLimit(contractAddr, abiStr, method, 0, args...)
}

// InvokeContractWithGasLimit 使用指定的 gasLimit 调用合约
// 逻辑与 InvokeContract 相同，区别在于使用传入的 gasLimit 而不是客户端默认值
func (c *EthereumClient) InvokeContractWithGasLimit(contractAddr, abiStr, method string, gasLimit uint64, args ...interface{}) (string, error) {

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
		return "", fmt.Errorf("failed to CreateInputData in InvokeContractWithGasLimit: %v", err)
	}

	// 如果指定的 gasLimit 为 0，则通过 EstimateGas 动态估算
	if gasLimit == 0 {
		estimatedGas, estErr := c.httpClient.EstimateGas(c.ctx, ethereum.CallMsg{
			From:     c.fromAddress,
			To:       &toAddress,
			GasPrice: gasPrice,
			Data:     data,
		})
		if estErr != nil {
			return "", fmt.Errorf("failed to EstimateGas: %v", estErr)
		}
		// 在估算值基础上增加 20% 余量，防止因状态变化导致 gas 不足
		gasLimit = estimatedGas * 120 / 100
	}

	// 构造交易
	tx := types.NewTransaction(nonce, toAddress, nil, gasLimit, gasPrice, data)

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
	if c.cancel != nil {
		c.cancel()
	}

	c.wg.Wait()
	if c.httpClient != nil {
		c.httpClient.Close()
	}
	if c.wsClient != nil {
		c.wsClient.Close()
	}
	return nil
}

// SubscribeContractEvent 订阅合约事件
func (c *EthereumClient) SubscribeContractEvent(contractConf ContractConf, chainConfName,
	contractConfName, chainType string, chainConfigID, contractConfigID uint) error {

	// 日志通用信息
	logFields := BuildSubscribeLogFields(map[string]interface{}{
		"chainConfName":    chainConfName,
		"contractConfName": contractConfName,
		"contractAddr":     contractConf.ContractAddr,
		"contractAbi":      contractConf.Abi,
		"module":           "subscribeEth",
	})

	// 检查合约地址不为空
	if contractConf.ContractAddr == "" {
		c.Logger.WithFields(logFields...).Error("eth contract address empty")
		return errors.New("eth contract address empty")
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
	} else {

		// redis 中存储的是处理过的最新高度，新的处理需要+1
		height++
	}

	logFields = append(logFields, logx.Field("startHeight", height))
	c.Logger.WithFields(logFields...).Infof("success to GetLatestBlockHeight %d for eth chain %s contract %s ", height,
		chainConfName, contractConfName)

	// 等待订阅协程
	c.wg.Add(1)
	defer c.wg.Done()

	// 实时订阅合约事件
	err = c.GetHistoryEvent(contractConf.ContractAddr, chainConfName, contractConfName, chainType, height,
		contractConf.GetHistoryEventHeightWindow, contractConf.GetHistoryEventInterval, logFields,
		chainConfigID, contractConfigID, blockHeightKey)
	if err != nil {
		c.Logger.WithFields(logFields...).Errorf("failed to GetHistoryEvent: %v", err)
		return fmt.Errorf("failed to GetHistoryEvent: %v", err)
	}

	return nil
}

// GetHistoryEvent 获取历史合约事件，实时事件可能会因为链分叉重组而移除，实时事件不是最终的，历史的比较准确
func (c *EthereumClient) GetHistoryEvent(contractAddr, chainConfName, contractConfName, chainType string,
	startHeight, window, interval uint64, logFields []logx.LogField,
	chainConfigID, contractConfigID uint, blockHeightKey string) error {

	// 定时去获取一次历史合约事件，以太坊 2.0 是 12s 出一个块
	// 防御性检查：interval 为 0 时使用默认值 12000ms（以太坊出块间隔）
	if interval == 0 {
		interval = 12000
	}

	// window 为 0 时使用默认值 100
	if window == 0 {
		window = 100
	}

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
				err = c.redisClient.SetLatestBlockHeight(c.ctx, blockHeightKey, endHeight)
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
				if err = c.redisClient.PublishCrossChainEventToStream(c.ctx, vLog,
					chainConfigID, contractConfigID, eventName); err != nil {
					c.Logger.WithFields(logFields...).Errorf("failed to publish eth event to redis stream: %v", err)
					break
				}
				c.Logger.WithFields(logFields...).Infof("publish eth contract event to redis stream: %#v", vLog)

				// 更新处理高度，以及下一次 startHeight
				if vLog.BlockNumber >= startHeight {
					err = c.redisClient.SetLatestBlockHeight(c.ctx, blockHeightKey, vLog.BlockNumber)
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
func (c *EthereumClient) RealTimeEvent(contractAddr, chainConfName, contractConfName, chainType string,
	height uint64, logFields []logx.LogField,
	chainConfigID, contractConfigID uint, blockHeightKey string) error {

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
			if err = c.redisClient.PublishCrossChainEventToStream(c.ctx, vLog,
				chainConfigID, contractConfigID, eventName); err != nil {
				c.Logger.WithFields(logFields...).Errorf("failed to publish event to redis stream: %v", err)
				return err
			}
			c.Logger.WithFields(logFields...).Infof("publish eth contract event to stream: %#v", vLog)

			// 更新最新区块高度
			if vLog.BlockNumber > height {
				err = c.redisClient.SetLatestBlockHeight(c.ctx, blockHeightKey, vLog.BlockNumber)
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

func (c *EthereumClient) CreateArgs(method string, kvs []*pb.KeyValuePair, abiStr string) ([]interface{}, error) {
	params := make(map[string][]byte, 0)
	for _, kv := range kvs {
		params[kv.Key] = kv.Value
	}

	// 通用 ABI 参数解析：根据 ABI 定义动态构建参数
	return c.createArgsFromABI(method, params, abiStr)
}

// createArgsFromABI 基于 ABI 定义动态解析合约方法参数
func (c *EthereumClient) createArgsFromABI(method string, params map[string][]byte, abiStr string) ([]interface{}, error) {
	if abiStr == "" {
		return nil, fmt.Errorf("abi string is empty, cannot parse args for method: %s", method)
	}

	contractABI, err := abi.JSON(strings.NewReader(abiStr))
	if err != nil {
		return nil, fmt.Errorf("failed to parse ABI: %v", err)
	}

	// 查找方法定义
	abiMethod, ok := contractABI.Methods[method]
	if !ok {
		return nil, fmt.Errorf("method %s not found in ABI", method)
	}

	// 如果方法没有参数，直接返回空列表
	if len(abiMethod.Inputs) == 0 {
		return []interface{}{}, nil
	}

	// 根据 ABI 参数定义，按顺序从 params 中提取并转换参数
	args := make([]interface{}, 0, len(abiMethod.Inputs))
	for _, input := range abiMethod.Inputs {
		rawValue, exists := params[input.Name]
		if !exists {
			return nil, fmt.Errorf("missing parameter: %s for method: %s", input.Name, method)
		}

		// 根据 ABI 类型转换参数值
		arg, err := convertABIParam(input.Type.String(), rawValue)
		if err != nil {
			return nil, fmt.Errorf("failed to convert param %s (type %s): %v", input.Name, input.Type.String(), err)
		}
		args = append(args, arg)
	}

	return args, nil
}

// convertABIParam 根据 ABI 类型字符串将原始字节值转换为对应的 Go 类型
func convertABIParam(abiType string, rawValue []byte) (interface{}, error) {
	strValue := string(rawValue)

	switch {
	// uint256, uint128, uint64 等大整数类型
	case strings.HasPrefix(abiType, "uint256") || strings.HasPrefix(abiType, "int256"):
		n := new(big.Int)
		_, ok := n.SetString(strValue, 10)
		if !ok {
			// 尝试十六进制
			_, ok = n.SetString(strings.TrimPrefix(strValue, "0x"), 16)
			if !ok {
				return nil, fmt.Errorf("invalid big integer value: %s", strValue)
			}
		}
		return n, nil

	case abiType == "uint8":
		val, err := parseUint(strValue, 8)
		if err != nil {
			return nil, err
		}
		return uint8(val), nil

	case abiType == "uint16":
		val, err := parseUint(strValue, 16)
		if err != nil {
			return nil, err
		}
		return uint16(val), nil

	case abiType == "uint32":
		val, err := parseUint(strValue, 32)
		if err != nil {
			return nil, err
		}
		return uint32(val), nil

	case abiType == "uint64":
		val, err := parseUint(strValue, 64)
		if err != nil {
			return nil, err
		}
		return val, nil

	case abiType == "int8":
		val, err := parseInt(strValue, 8)
		if err != nil {
			return nil, err
		}
		return int8(val), nil

	case abiType == "int16":
		val, err := parseInt(strValue, 16)
		if err != nil {
			return nil, err
		}
		return int16(val), nil

	case abiType == "int32":
		val, err := parseInt(strValue, 32)
		if err != nil {
			return nil, err
		}
		return int32(val), nil

	case abiType == "int64":
		val, err := parseInt(strValue, 64)
		if err != nil {
			return nil, err
		}
		return val, nil

	// address 类型
	case abiType == "address":
		return common.HexToAddress(strValue), nil

	// bool 类型
	case abiType == "bool":
		switch strings.ToLower(strValue) {
		case "true", "1":
			return true, nil
		case "false", "0":
			return false, nil
		default:
			return nil, fmt.Errorf("invalid bool value: %s", strValue)
		}

	// string 类型
	case abiType == "string":
		return strValue, nil

	// bytes32, bytes20 等固定长度字节数组
	case strings.HasPrefix(abiType, "bytes") && abiType != "bytes":
		return common.FromHex(strValue), nil

	// bytes 动态字节数组
	case abiType == "bytes":
		return rawValue, nil

	default:
		// 对于不认识的类型，尝试直接传字符串
		return strValue, nil
	}
}

// parseUint 解析无符号整数
func parseUint(s string, bitSize int) (uint64, error) {
	val, err := fmt.Sscanf(s, "%d", new(uint64))
	if err != nil || val == 0 {
		return 0, fmt.Errorf("invalid uint%d value: %s", bitSize, s)
	}
	var result uint64
	_, err = fmt.Sscanf(s, "%d", &result)
	return result, err
}

// parseInt 解析有符号整数
func parseInt(s string, bitSize int) (int64, error) {
	val, err := fmt.Sscanf(s, "%d", new(int64))
	if err != nil || val == 0 {
		return 0, fmt.Errorf("invalid int%d value: %s", bitSize, s)
	}
	var result int64
	_, err = fmt.Sscanf(s, "%d", &result)
	return result, err
}
