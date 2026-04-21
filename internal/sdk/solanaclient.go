package sdk

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
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

	// RPC 连接
	rpcClient     *rpc.Client
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

	// 建立 RPC 连接
	rpcClient := rpc.New(solanaConf.RpcUrl)

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

// CallContract 调用合约
func (c *SolanaClient) CallContract(methodType pb.MethodType, contractConfigName, method string,
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
						goto confirmed
					}

				case <-time.After(time.Duration(txTimeout) * time.Second):
					txBytes, err2 := json.Marshal(txResp)
					if err2 != nil {
						return "", "", fmt.Errorf("failed to marshal transaction response: %v", err2)
					}
					return txId, string(txBytes), fmt.Errorf("sync to get transaction confirmation timeout")
				}
			}
		confirmed:
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
	if c.rpcClient != nil {
		if err := c.rpcClient.Close(); err != nil {
			return err
		}
	}
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

	// 使用轮询方式获取合约事件（不采用 WebSocket 实时订阅，因为实时方式无法控制连续同步，可能出现事件遗漏）
	go c.getHistoryEvents(chainConfName, contractConfName, chainType, contractType, height, logFields)

	c.Logger.WithFields(logFields...).Info("Solana event subscription started successfully")

	return nil
}

// getHistoryEvents 轮询获取历史事件
// 使用 GetSignaturesForAddressWithOpts 轮询合约相关的已确认交易签名，
// 然后获取每笔交易的详情来解析事件数据。
// 注意：GetSignaturesForAddress API 是按签名从新到旧遍历的，不支持按 slot 范围过滤，
// MinContextSlot 只是保证查询在某个最低 slot 高度上执行，不是过滤条件。
// 因此正确的做法是：每次轮询获取最近的签名，过滤出 slot >= currentSlot 的新签名，
// 按 slot 从小到大排序处理，确保事件不遗漏且有序。
func (c *SolanaClient) getHistoryEvents(chainConfName, contractConfName,
	chainType, contractType string, startSlot uint64, logFields []logx.LogField) {

	contractConf, ok := c.contractConfigs[contractConfName]
	if !ok {
		c.Logger.WithFields(logFields...).Errorf("contract config not found: %s", contractConfName)
		return
	}

	// 解析合约地址（Program ID）
	programID, err := solana.PublicKeyFromBase58(contractConf.ContractAddr)
	if err != nil {
		c.Logger.WithFields(logFields...).Errorf("invalid contract address: %v", err)
		return
	}

	// 轮询间隔：默认 5 秒
	interval := time.Duration(contractConf.GetHistoryEventInterval) * time.Millisecond
	if interval == 0 {
		interval = 5000 * time.Millisecond
	}

	// 每次查询返回的最大签名数量：默认 1000（API 最大值）
	queryLimit := contractConf.GetHistoryEventHeightWindow
	if queryLimit == 0 {
		queryLimit = 1000
	}

	key := strings.Join([]string{chainType, chainConfName, contractType, contractConfName}, "#")
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	currentSlot := startSlot

	for {
		select {
		case <-c.ctx.Done():
			c.Logger.WithFields(logFields...).Info("getHistoryEvents context done, exiting")
			return
		case <-ticker.C:
			// 获取当前链的最新 slot 高度
			latestSlot, slotErr := c.rpcClient.GetSlot(c.ctx, c.commitment)
			if slotErr != nil {
				c.Logger.WithFields(logFields...).Errorf("failed to get latest slot: %v", slotErr)
				continue
			}

			// 如果当前处理位置已追上最新高度，则跳过本次轮询
			if currentSlot > latestSlot {
				continue
			}

			// 收集所有 slot >= currentSlot 的新签名
			newSigs := c.fetchNewSignatures(programID, currentSlot, queryLimit, logFields)

			// 如果没有新签名，推进 currentSlot 到 latestSlot + 1，避免重复查询
			if len(newSigs) == 0 {
				if setErr := c.redisClient.SetLatestBlockHeight(c.ctx, key, latestSlot); setErr != nil {
					c.Logger.WithFields(logFields...).Errorf("failed to set latest block height: %v", setErr)
				} else {
					c.Logger.WithFields(logFields...).Infof("no new events, updated slot to %d", latestSlot)
				}
				currentSlot = latestSlot + 1
				continue
			}

			// 按 slot 从小到大排序，确保事件按顺序处理
			sortTransactionSignatures(newSigs)

			c.Logger.WithFields(logFields...).Infof("found %d new signatures since slot %d", len(newSigs), currentSlot)

			// 处理每笔交易的事件
			processedSlot := c.processTransactionSignatures(newSigs, currentSlot,
				chainConfName, contractConfName, chainType, contractType, logFields)

			// 更新 Redis 中已处理的 slot 高度
			if processedSlot > currentSlot {
				if setErr := c.redisClient.SetLatestBlockHeight(c.ctx, key, processedSlot); setErr != nil {
					c.Logger.WithFields(logFields...).Errorf("failed to set latest block height: %v", setErr)
				} else {
					c.Logger.WithFields(logFields...).Infof("updated processed slot to %d", processedSlot)
				}
				currentSlot = processedSlot + 1
			}
		}
	}
}

// fetchNewSignatures 获取所有 slot >= currentSlot 的新交易签名
// 由于 GetSignaturesForAddress API 从新到旧返回，可能需要分页获取
func (c *SolanaClient) fetchNewSignatures(programID solana.PublicKey, currentSlot uint64,
	queryLimit uint64, logFields []logx.LogField) []*rpc.TransactionSignature {

	var newSigs []*rpc.TransactionSignature
	var beforeSig solana.Signature

	for {
		opts := &rpc.GetSignaturesForAddressOpts{
			Commitment: c.commitment,
			Limit:      ptrInt(int(queryLimit)),
		}

		// 如果有分页参数，设置 Before 从上一批最旧的签名继续往前查
		if !beforeSig.IsZero() {
			opts.Before = beforeSig
		}

		signatures, err := c.rpcClient.GetSignaturesForAddressWithOpts(
			c.ctx,
			programID,
			opts,
		)
		if err != nil {
			c.Logger.WithFields(logFields...).Errorf("failed to get signatures for address: %v", err)
			break
		}

		if len(signatures) == 0 {
			break
		}

		// 过滤出 slot >= currentSlot 的签名
		hasOldSigs := false
		for _, sig := range signatures {
			if sig.Slot >= currentSlot {
				newSigs = append(newSigs, sig)
			} else {
				hasOldSigs = true
			}
		}

		// 如果已经遇到旧签名，说明已遍历到 currentSlot 之前，无需继续分页
		if hasOldSigs {
			break
		}

		// 如果返回数量小于查询限制，说明已获取所有签名，无需分页
		if len(signatures) < int(queryLimit) {
			break
		}

		// 需要继续分页，使用本批最旧的签名作为下次查询的 Before 参数
		beforeSig = signatures[len(signatures)-1].Signature
	}

	return newSigs
}

// processTransactionSignatures 处理交易签名列表，返回已处理的最大 slot
func (c *SolanaClient) processTransactionSignatures(sigs []*rpc.TransactionSignature, currentSlot uint64,
	chainConfName, contractConfName, chainType, contractType string, logFields []logx.LogField) uint64 {

	processedSlot := currentSlot
	for _, txSig := range sigs {
		// 获取交易详情
		txResult, err := c.rpcClient.GetTransaction(c.ctx, txSig.Signature, &rpc.GetTransactionOpts{
			Commitment: c.commitment,
		})
		if err != nil {
			c.Logger.WithFields(logFields...).Errorf("failed to get transaction %s: %v", txSig.Signature.String(), err)
			continue
		}

		if txResult == nil {
			continue
		}

		// 解析交易日志并发布事件
		c.processAndPublishEvent(txResult, txSig.Signature.String(), txSig.Slot,
			chainConfName, contractConfName, chainType, contractType, logFields)

		// 记录已处理的最大 slot
		if txSig.Slot > processedSlot {
			processedSlot = txSig.Slot
		}
	}
	return processedSlot
}

// sortTransactionSignatures 按 slot 从小到大排序交易签名，确保事件按区块顺序处理
func sortTransactionSignatures(sigs []*rpc.TransactionSignature) {
	sort.Slice(sigs, func(i, j int) bool {
		return sigs[i].Slot < sigs[j].Slot
	})
}

// processAndPublishEvent 解析交易结果并发布事件到 Redis
func (c *SolanaClient) processAndPublishEvent(txResult *rpc.GetTransactionResult, signature string, slot uint64,
	chainConfName, contractConfName, chainType, contractType string, logFields []logx.LogField) {

	if txResult == nil || txResult.Meta == nil {
		return
	}

	// 构建事件数据
	eventData := map[string]interface{}{
		"signature": signature,
		"slot":      slot,
		"logs":      txResult.Meta.LogMessages,
		"err":       txResult.Meta.Err,
	}

	// 如果有区块时间，也加入事件数据
	if txResult.BlockTime != nil {
		eventData["blockTime"] = txResult.BlockTime.Time().Unix()
	}

	eventBytes, err := json.Marshal(eventData)
	if err != nil {
		c.Logger.WithFields(logFields...).Errorf("failed to marshal event data: %v", err)
		return
	}

	// 发布事件到 Redis
	err = c.redisClient.PublishTradeGuardEventToStream(c.ctx, string(eventBytes), chainType,
		chainConfName, contractConfName, contractType,
		c.contractConfigs[contractConfName].ContractType)
	if err != nil {
		c.Logger.WithFields(logFields...).Errorf("failed to publish event to redis: %v", err)
		return
	}

	c.Logger.WithFields(logFields...).Infof("published history event: signature=%s, slot=%d", signature, slot)
}

// ptrInt 返回 int 的指针，用于 rpc.GetSignaturesForAddressOpts 的 Limit 字段
func ptrInt(v int) *int {
	return &v
}
