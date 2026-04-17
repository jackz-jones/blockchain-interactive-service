package sdk

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/rpc"
	"github.com/jackz-jones/blockchain-interactive-service/internal/config"
	pb "github.com/jackz-jones/blockchain-interactive-service/pb"
	commonEvent "github.com/jackz-jones/common/event"
	"github.com/zeromicro/go-zero/core/logx"
)

// SolanaClient 定义了 Solana 客户端对象
type SolanaClient struct {
	ctx context.Context

	// 合约配置，配置名称--》合约信息
	contractConfigs map[string]*config.ContractConf

	// RPC 和 WebSocket 连接
	rpcClient     *rpc.Client
	wsClient      *rpc.Client
	privateKey    solana.PrivateKey
	fromAddress   solana.PublicKey
	commitment    rpc.CommitmentType
	skipPreflight bool
	maxRetries    int
	logx.Logger

	// redis 客户端
	redisClient *commonEvent.RedisClient
}

// NewSolanaClient 创建一个 SolanaClient 对象
func NewSolanaClient(ctx context.Context, solanaConf config.SolanaConf, contractConfs map[string]*config.ContractConf,
	redisClient *commonEvent.RedisClient) (*SolanaClient, error) {

	// 验证 RPC URL 不为空
	if solanaConf.RpcUrl == "" {
		return nil, errors.New("rpc url is empty")
	}

	// 验证 WebSocket URL 不为空
	if solanaConf.WsUrl == "" {
		return nil, errors.New("ws url is empty")
	}

	// 建立 RPC 连接
	rpcClient := rpc.New(solanaConf.RpcUrl)

	// 建立 WebSocket 连接
	wsClient := rpc.New(solanaConf.WsUrl)

	// 解析私钥
	privateKey, err := solana.PrivateKeyFromBase58(solanaConf.PrivateKey)
	if err != nil {
		return nil, fmt.Errorf("failed to parse private key: %v", err)
	}

	// 获取公钥（在 Solana 中，公钥的 base58 编码即为地址，无需额外转换）
	fromAddress := privateKey.PublicKey()

	// 解析确认级别
	var commitment rpc.CommitmentType
	switch solanaConf.CommitmentLevel {
	case "processed":
		commitment = rpc.CommitmentProcessed
	case "confirmed":
		commitment = rpc.CommitmentConfirmed
	case "finalized":
		commitment = rpc.CommitmentFinalized
	default:
		commitment = rpc.CommitmentConfirmed
	}

	return &SolanaClient{
		ctx:             ctx,
		contractConfigs: contractConfs,
		rpcClient:       rpcClient,
		wsClient:        wsClient,
		privateKey:      privateKey,
		fromAddress:     fromAddress,
		commitment:      commitment,
		skipPreflight:   solanaConf.SkipPreflight,
		maxRetries:      solanaConf.MaxRetries,
		Logger:          logx.WithContext(ctx),
		redisClient:     redisClient,
	}, nil
}

// GetTxByTxId 根据交易ID查询交易
func (c *SolanaClient) GetTxByTxId(txId string) (string, bool, error) {
	// 解析交易签名
	signature, err := solana.SignatureFromBase58(txId)
	if err != nil {
		return "", false, fmt.Errorf("invalid transaction signature: %v", err)
	}

	// 查询交易状态
	txStatus, err := c.rpcClient.GetSignatureStatuses(c.ctx, false, signature)
	if err != nil {
		return "", false, fmt.Errorf("failed to get signature status: %v", err)
	}

	// 检查交易是否存在
	if txStatus == nil || len(txStatus.Value) == 0 || txStatus.Value[0] == nil {
		return "", false, fmt.Errorf("transaction not found")
	}

	status := txStatus.Value[0]

	// 检查交易是否已确认
	isPending := status.ConfirmationStatus == rpc.ConfirmationStatusProcessed || status.ConfirmationStatus == ""

	// 构建交易响应
	txResp := map[string]interface{}{
		"signature":     txId,
		"slot":          status.Slot,
		"confirmations": status.Confirmations,
		"status":        status,
		"err":           status.Err,
	}

	txBytes, err := json.Marshal(txResp)
	if err != nil {
		return "", false, fmt.Errorf("failed to marshal transaction response: %v", err)
	}

	return string(txBytes), isPending, nil
}

// SendTransaction 发送交易
func (c *SolanaClient) SendTransaction(methodType pb.MethodType, contractConfigName, method string,
	args []*pb.KeyValuePair, txTimeout int64, withSyncResult bool) (string, string, error) {

	var (
		txResp interface{}
		err    error
		txId   string
	)

	// 获取合约配置
	contractConf, ok := c.contractConfigs[contractConfigName]
	if !ok {
		return "", "", fmt.Errorf("unknown contract config name %s", contractConfigName)
	}

	// 解析合约地址
	contractAddress, err := solana.PublicKeyFromBase58(contractConf.ContractAddr)
	if err != nil {
		return "", "", fmt.Errorf("invalid contract address: %v", err)
	}

	switch methodType {
	case pb.MethodType_Invoke:
		// 调用合约方法
		txId, err = c.invokeContract(contractAddress, method, args)
		if err != nil {
			return "", "", fmt.Errorf("failed to invoke contract: %v", err)
		}

		// Solana 的交易 ID（Transaction Signature）就是交易签名的 base58 编码字符串
		// 构建响应
		txResp = map[string]string{"signature": txId}

		if withSyncResult {
			// 同步等待交易确认
			ticker := time.NewTicker(time.Second)
			defer ticker.Stop()

			for {
				select {
				case <-ticker.C:
					// 检查交易状态
					_, isPending, err2 := c.GetTxByTxId(txId)
					if err2 != nil {
						c.Logger.Errorf("failed to get transaction status: %v", err2)
						continue
					}

					if !isPending {
						// 交易已确认，获取详细交易信息
						txDetails, err3 := c.rpcClient.GetTransaction(c.ctx, solana.MustSignatureFromBase58(txId), nil)
						if err3 != nil {
							c.Logger.Errorf("failed to get transaction details: %v", err3)
							continue
						}
						txResp = txDetails
						break
					}

				case <-time.After(time.Duration(txTimeout) * time.Second):
					txBytes, err2 := json.Marshal(txResp)
					if err2 != nil {
						return "", "", fmt.Errorf("failed to marshal transaction response: %v", err2)
					}
					return txId, string(txBytes), fmt.Errorf("sync to get transaction confirmation timeout")
				}
			}
		}

	case pb.MethodType_Query:
		// 查询合约状态
		txResp, err = c.queryContract(contractAddress, method, args)
		if err != nil {
			return "", "", fmt.Errorf("failed to query contract: %v", err)
		}

	default:
		return "", "", fmt.Errorf("unsupported method type %d", methodType)
	}

	txBytes, err := json.Marshal(txResp)
	if err != nil {
		return txId, "", fmt.Errorf("failed to marshal transaction response: %v", err)
	}

	return txId, string(txBytes), nil
}

// invokeContract 调用合约方法
func (c *SolanaClient) invokeContract(contractAddress solana.PublicKey, method string,
	args []*pb.KeyValuePair) (string, error) {
	// 获取最新区块哈希
	latestBlockhash, err := c.rpcClient.GetLatestBlockhash(c.ctx, c.commitment)
	if err != nil {
		return "", fmt.Errorf("failed to get latest blockhash: %v", err)
	}

	// 构建指令数据（这里需要根据具体合约ABI实现）
	data, err := c.createInstructionData(method, args)
	if err != nil {
		return "", fmt.Errorf("failed to create instruction data: %v", err)
	}

	// 创建账户元数据
	accounts := solana.AccountMetaSlice{
		{PublicKey: contractAddress, IsWritable: true, IsSigner: false},
		{PublicKey: c.fromAddress, IsWritable: true, IsSigner: true},
	}

	// 创建指令
	instruction := solana.NewInstruction(
		contractAddress,
		accounts,
		data,
	)

	// 构建交易
	tx, err := solana.NewTransaction(
		[]solana.Instruction{instruction},
		latestBlockhash.Value.Blockhash,
		solana.TransactionPayer(c.fromAddress),
	)
	if err != nil {
		return "", fmt.Errorf("failed to create transaction: %v", err)
	}

	// 签名交易
	_, err = tx.Sign(func(key solana.PublicKey) *solana.PrivateKey {
		if key.Equals(c.fromAddress) {
			return &c.privateKey
		}
		return nil
	})
	if err != nil {
		return "", fmt.Errorf("failed to sign transaction: %v", err)
	}

	// 发送交易
	signature, err := c.rpcClient.SendTransaction(c.ctx, tx)
	if err != nil {
		return "", fmt.Errorf("failed to send transaction: %v", err)
	}

	return signature.String(), nil
}

// queryContract 查询合约状态
func (c *SolanaClient) queryContract(contractAddress solana.PublicKey, method string,
	args []*pb.KeyValuePair) (interface{}, error) {
	// 构建指令数据
	data, err := c.createInstructionData(method, args)
	if err != nil {
		return nil, fmt.Errorf("failed to create instruction data: %v", err)
	}

	// 创建账户元数据
	accounts := solana.AccountMetaSlice{
		{PublicKey: contractAddress, IsWritable: false, IsSigner: false},
		{PublicKey: c.fromAddress, IsWritable: false, IsSigner: false},
	}

	// 创建指令
	instruction := solana.NewInstruction(
		contractAddress,
		accounts,
		data,
	)

	// 获取最新区块哈希
	latestBlockhash, err := c.rpcClient.GetLatestBlockhash(c.ctx, c.commitment)
	if err != nil {
		return nil, fmt.Errorf("failed to get latest blockhash: %v", err)
	}

	// 构建模拟交易
	tx, err := solana.NewTransaction(
		[]solana.Instruction{instruction},
		latestBlockhash.Value.Blockhash,
		solana.TransactionPayer(c.fromAddress),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create transaction: %v", err)
	}

	// 模拟交易来查询状态
	result, err := c.rpcClient.SimulateTransaction(c.ctx, tx)
	if err != nil {
		return nil, fmt.Errorf("failed to simulate transaction: %v", err)
	}

	return result, nil
}

// createInstructionData 创建指令数据
func (c *SolanaClient) createInstructionData(method string, args []*pb.KeyValuePair) ([]byte, error) {
	// 这里需要根据具体合约的指令格式来构建数据
	// 简单实现：将方法和参数编码为JSON
	instruction := map[string]interface{}{
		"method": method,
		"args":   args,
	}

	data, err := json.Marshal(instruction)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal instruction: %v", err)
	}

	return data, nil
}

// Stop 停止客户端
func (c *SolanaClient) Stop() error {
	// Solana RPC 客户端没有明确的关闭方法
	return nil
}

// SubscribeContractEvent 订阅合约事件
func (c *SolanaClient) SubscribeContractEvent(contractConf config.ContractConf, chainConfName,
	contractConfName, chainType, contractType string) error {

	// 日志通用信息
	fields := map[string]interface{}{
		"chainConfName":    chainConfName,
		"contractConfName": contractConfName,
		"contractAddr":     contractConf.ContractAddr,
		"contractType":     contractConf.ContractType,
		"module":           "subscribeSolana",
	}
	logFields := make([]logx.LogField, 0)
	for k, v := range fields {
		logFields = append(logFields, logx.Field(k, v))
	}

	// 检查合约地址不为空
	if contractConf.ContractAddr == "" {
		c.Logger.WithFields(logFields...).Error("solana contract address empty")
		return errors.New("solana contract address empty")
	}

	// 解析合约地址
	_, err := solana.PublicKeyFromBase58(contractConf.ContractAddr)
	if err != nil {
		c.Logger.WithFields(logFields...).Errorf("invalid contract address: %v", err)
		return fmt.Errorf("invalid contract address: %v", err)
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

	logFields = append(logFields, logx.Field("startSlot", height))
	c.Logger.WithFields(logFields...).Infof("success to GetLatestBlockHeight %d for solana chain %s contract %s", height,
		chainConfName, contractConfName)

	// 订阅日志事件（Solana 使用日志订阅来监听合约事件）
	// TODO: 实现 Solana 事件订阅功能
	// 目前先返回成功，后续需要根据 solana-go 库的API实现具体订阅逻辑
	c.Logger.WithFields(logFields...).Info("Solana event subscription not implemented yet")

	return nil
}

// handleLogEvents 处理日志事件
func (c *SolanaClient) handleLogEvents(chainConfName, contractConfName,
	chainType, contractType string, startSlot uint64, logFields []logx.LogField) {
	// TODO: 实现 Solana 事件处理逻辑
	c.Logger.WithFields(logFields...).Info("Solana event handling not implemented yet")
}
