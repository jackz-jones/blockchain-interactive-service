package sdk

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/rpc"
	"github.com/jackz-jones/blockchain-interactive-service/internal/code"
	"github.com/jackz-jones/blockchain-interactive-service/internal/config"
	pb "github.com/jackz-jones/blockchain-interactive-service/pb"
	commonEvent "github.com/jackz-jones/common/event"
	"github.com/zeromicro/go-zero/core/logx"
)

// solanaMaxFetchSignaturesPages 每轮拉取事件签名的最大翻页次数，防止无限翻页
const solanaMaxFetchSignaturesPages = 50

// solanaDefaultMaxRetries SendTransactionOpts.MaxRetries 的默认值
const solanaDefaultMaxRetries uint = 3

// SolanaClient 定义了 Solana 客户端对象
type SolanaClient struct {
	// ctx 由外部传入，用作客户端根上下文（Stop 后会被 cancel）
	ctx    context.Context
	cancel context.CancelFunc

	// wg 用于等待订阅 goroutine 退出
	wg sync.WaitGroup

	// 合约配置，配置名称--》合约信息
	contractConfigs map[string]*config.ContractConf

	// RPC 连接
	rpcClient     *rpc.Client
	privateKey    solana.PrivateKey
	fromAddress   solana.PublicKey
	commitment    rpc.CommitmentType
	skipPreflight bool
	maxRetries    uint
	logx.Logger

	// txFetcher 允许测试注入自定义交易读取器以避免真实链依赖；
	// 生产时指向 rpcClient.GetTransaction。
	txFetcher func(ctx context.Context, sig solana.Signature,
		opts *rpc.GetTransactionOpts) (*rpc.GetTransactionResult, error)

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

	// 基于父 ctx 派生可取消子 ctx，用于订阅 goroutine 的生命周期控制
	childCtx, cancel := context.WithCancel(ctx)

	// MaxRetries 默认值处理
	maxRetries := uint(solanaConf.MaxRetries)
	logger := logx.WithContext(childCtx)
	if maxRetries == 0 {
		maxRetries = solanaDefaultMaxRetries
		logger.Infof("solana MaxRetries not configured, use default: %d", maxRetries)
	}

	client := &SolanaClient{
		ctx:             childCtx,
		cancel:          cancel,
		contractConfigs: contractConfs,
		rpcClient:       rpcClient,
		privateKey:      privateKey,
		fromAddress:     fromAddress,
		commitment:      commitment,
		skipPreflight:   solanaConf.SkipPreflight,
		maxRetries:      maxRetries,
		Logger:          logger,
		redisClient:     redisClient,
	}
	// 默认交易读取器指向真实的 rpcClient.GetTransaction
	client.txFetcher = rpcClient.GetTransaction
	return client, nil
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

	// 查找方法规范（Invoke/Query 均必须声明）
	methodSpec, specOK := contractConf.SolanaMethods[method]
	if !specOK {
		return "", "", fmt.Errorf("solana method spec for [%s.%s] not configured", contractConfigName, method)
	}

	switch methodType {
	case pb.MethodType_Invoke:
		// 调用合约方法
		txId, err = c.invokeContract(contractAddress, methodSpec, args)
		if err != nil {
			return "", "", fmt.Errorf("failed to invoke contract: %v", err)
		}

		// Solana 的交易 ID（Transaction Signature）就是交易签名的 base58 编码字符串
		// 构建默认响应
		txResp = map[string]string{"signature": txId}

		if withSyncResult {
			// 同步等待交易确认：使用 context.WithTimeout + ticker 轮询，
			// 不再使用 goto；超时返回 code.ErrGetTxReceiptTimeoutMsg 以保持与 Ethereum 语义一致。
			confirmed, confirmResp, confirmErr := c.waitForTransactionConfirmation(txId, txTimeout)
			if confirmErr != nil {
				txBytes, _ := json.Marshal(txResp)
				return txId, string(txBytes), confirmErr
			}
			if confirmed {
				txResp = confirmResp
			}
		}

	case pb.MethodType_Query:
		// 查询合约状态
		txResp, err = c.queryContract(methodSpec)
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

// waitForTransactionConfirmation 轮询等待交易确认，返回 (是否已确认, 交易详情, 错误)。
// 超时错误信息包含 code.ErrGetTxReceiptTimeoutMsg，上层可以识别以做重试/降级处理。
func (c *SolanaClient) waitForTransactionConfirmation(txId string, txTimeout int64) (bool, interface{}, error) {
	waitCtx, cancel := context.WithTimeout(c.ctx, time.Duration(txTimeout)*time.Second)
	defer cancel()

	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	signature := solana.MustSignatureFromBase58(txId)
	for {
		select {
		case <-waitCtx.Done():
			// 区分：父 ctx 取消 vs 超时
			if errors.Is(waitCtx.Err(), context.DeadlineExceeded) {
				return false, nil, fmt.Errorf("%s, txId: %s", code.ErrGetTxReceiptTimeoutMsg, txId)
			}
			return false, nil, fmt.Errorf("wait transaction canceled: %v", waitCtx.Err())

		case <-ticker.C:
			_, isPending, err := c.GetTxByTxId(txId)
			if err != nil {
				c.Logger.Errorf("failed to get transaction status: %v", err)
				continue
			}
			if isPending {
				continue
			}

			// 交易已确认，获取详细交易信息
			txDetails, detailErr := c.rpcClient.GetTransaction(c.ctx, signature, nil)
			if detailErr != nil {
				c.Logger.Errorf("failed to get transaction details: %v", detailErr)
				continue
			}
			return true, txDetails, nil
		}
	}
}

// invokeContract 调用合约方法（写链）
func (c *SolanaClient) invokeContract(contractAddress solana.PublicKey, methodSpec config.SolanaMethodSpec,
	args []*pb.KeyValuePair) (string, error) {

	// 获取最新区块哈希
	latestBlockhash, err := c.rpcClient.GetLatestBlockhash(c.ctx, c.commitment)
	if err != nil {
		return "", fmt.Errorf("failed to get latest blockhash: %v", err)
	}

	// 按方法规范构建 Borsh + discriminator 指令数据
	data, err := EncodeInstructionData(methodSpec, args)
	if err != nil {
		return "", fmt.Errorf("failed to encode instruction data: %v", err)
	}

	// 按方法规范构建账户列表；若未配置则 WARN 并使用默认
	accounts, usedDefault, err := BuildAccountMetaSlice(methodSpec.Accounts, c.fromAddress)
	if err != nil {
		return "", fmt.Errorf("failed to build account meta slice: %v", err)
	}
	if usedDefault {
		c.Logger.Errorf("solana method accounts not configured, use default [fromAddress(signer,writable)]; "+
			"this may fail if contract requires specific accounts, programID=%s", contractAddress.String())
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

	// 发送交易：应用 SkipPreflight / MaxRetries / PreflightCommitment 配置
	maxRetries := c.maxRetries
	sendOpts := rpc.TransactionOpts{
		SkipPreflight:       c.skipPreflight,
		PreflightCommitment: c.commitment,
		MaxRetries:          &maxRetries,
	}
	signature, err := c.rpcClient.SendTransactionWithOpts(c.ctx, tx, sendOpts)
	if err != nil {
		return "", fmt.Errorf("failed to send transaction: %v", err)
	}

	return signature.String(), nil
}

// queryContract 查询合约状态（读链）
// Solana 合约的状态存在 "账户数据" 里，正确的读取方式是 GetMultipleAccounts 而非 SimulateTransaction。
func (c *SolanaClient) queryContract(methodSpec config.SolanaMethodSpec) (interface{}, error) {
	if len(methodSpec.QueryAccounts) == 0 {
		return nil, errors.New("solana query method requires non-empty QueryAccounts in method spec")
	}

	pubkeys := make([]solana.PublicKey, 0, len(methodSpec.QueryAccounts))
	for i, raw := range methodSpec.QueryAccounts {
		pk, err := resolvePubkey(raw, c.fromAddress)
		if err != nil {
			return nil, fmt.Errorf("queryAccount[%d] invalid pubkey: %v", i, err)
		}
		pubkeys = append(pubkeys, pk)
	}

	// 使用 GetMultipleAccounts 一次性拉取账户数据
	resp, err := c.rpcClient.GetMultipleAccounts(c.ctx, pubkeys...)
	if err != nil {
		return nil, fmt.Errorf("failed to get multiple accounts: %v", err)
	}
	if resp == nil || resp.Value == nil {
		return nil, errors.New("get multiple accounts returned empty value")
	}

	// 构建响应结构
	type accountInfo struct {
		Pubkey     string `json:"pubkey"`
		Exists     bool   `json:"exists"`
		Owner      string `json:"owner,omitempty"`
		Lamports   uint64 `json:"lamports,omitempty"`
		Executable bool   `json:"executable,omitempty"`
		DataBase64 string `json:"dataBase64,omitempty"`
	}

	result := make([]accountInfo, 0, len(pubkeys))
	var notFound []string
	for i, pk := range pubkeys {
		item := accountInfo{Pubkey: pk.String()}
		if i >= len(resp.Value) || resp.Value[i] == nil {
			item.Exists = false
			notFound = append(notFound, pk.String())
			result = append(result, item)
			continue
		}
		acc := resp.Value[i]
		item.Exists = true
		item.Owner = acc.Owner.String()
		item.Lamports = acc.Lamports
		item.Executable = acc.Executable
		if acc.Data != nil {
			item.DataBase64 = base64.StdEncoding.EncodeToString(acc.Data.GetBinary())
		}
		result = append(result, item)
	}

	// 若存在账户不存在，返回明确错误（但响应结构仍可序列化为 JSON 供上层参考）
	if len(notFound) > 0 {
		return result, fmt.Errorf("account(s) not found: %s", strings.Join(notFound, ","))
	}

	return result, nil
}

// Stop 停止客户端：取消内部 ctx，等待订阅 goroutine 退出，然后关闭 RPC 连接。
func (c *SolanaClient) Stop() error {
	if c.cancel != nil {
		c.cancel()
	}
	c.wg.Wait()
	if c.rpcClient != nil {
		// rpc.Client.Close 释放底层 http transport
		return c.rpcClient.Close()
	}
	return nil
}

// SubscribeContractEvent 订阅合约事件（阻塞直到 ctx Done 或致命错误返回）。
// 语义与 Ethereum/ChainMaker 保持一致：调用方（StartSubscribe 的 goroutine）会阻塞，
// 若返回非 nil 错误，则 StartSubscribe 会清理 SubscribeFlag，在下一个 3 秒轮询触发重订阅。
func (c *SolanaClient) SubscribeContractEvent(contractConf config.ContractConf, chainConfName,
	contractConfName, chainType, contractType string) error {

	// 日志通用信息
	logFields := BuildSubscribeLogFields(map[string]interface{}{
		"chainConfName":    chainConfName,
		"contractConfName": contractConfName,
		"contractAddr":     contractConf.ContractAddr,
		"contractType":     contractConf.ContractType,
		"module":           "subscribeSolana",
	})

	// 检查合约地址不为空
	if contractConf.ContractAddr == "" {
		c.Logger.WithFields(logFields...).Error("solana contract address empty")
		return errors.New("solana contract address empty")
	}

	// 解析合约地址
	programID, err := solana.PublicKeyFromBase58(contractConf.ContractAddr)
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

	// 使用轮询方式获取合约事件（不采用 WebSocket 实时订阅，因为实时方式无法控制连续同步，可能出现事件遗漏）。
	// 此处改为同步阻塞调用，与 Ethereum/ChainMaker 实现保持一致。
	c.wg.Add(1)
	defer c.wg.Done()
	return c.getHistoryEvents(contractConf, chainConfName, contractConfName, chainType,
		contractType, height, logFields, programID)
}

// getHistoryEvents 轮询获取历史事件（同步阻塞）
// 使用 GetSignaturesForAddressWithOpts 轮询合约相关的已确认交易签名，
// 然后获取每笔交易的详情来解析事件数据。
func (c *SolanaClient) getHistoryEvents(contractConf config.ContractConf, chainConfName, contractConfName,
	chainType, contractType string, startSlot uint64, logFields []logx.LogField,
	programID solana.PublicKey) error {

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
			return nil
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

			// 处理每笔交易的事件；返回 (已处理的最大 slot, 是否所有签名都成功处理)
			processedSlot, allOK := c.processTransactionSignatures(newSigs, currentSlot,
				chainConfName, contractConfName, chainType, contractType, logFields)

			// 仅在 "本轮所有签名都成功处理" 时推进 currentSlot；
			// 只要有一笔失败，就停在失败 slot 之前，等待下一轮重试。
			if processedSlot > currentSlot {
				if setErr := c.redisClient.SetLatestBlockHeight(c.ctx, key, processedSlot); setErr != nil {
					c.Logger.WithFields(logFields...).Errorf("failed to set latest block height: %v", setErr)
					continue
				}
				c.Logger.WithFields(logFields...).Infof("updated processed slot to %d (allOK=%v)", processedSlot, allOK)
				currentSlot = processedSlot + 1
			} else if !allOK {
				c.Logger.WithFields(logFields...).Infof("some signatures failed, keep currentSlot=%d for retry", currentSlot)
			}
		}
	}
}

// fetchNewSignatures 获取所有 slot >= currentSlot 的新交易签名。
// 由于 GetSignaturesForAddress API 从新到旧返回，可能需要分页获取；
// 为避免极端情况下无限翻页，设置 solanaMaxFetchSignaturesPages 作为硬性上限。
func (c *SolanaClient) fetchNewSignatures(programID solana.PublicKey, currentSlot uint64,
	queryLimit uint64, logFields []logx.LogField) []*rpc.TransactionSignature {

	var newSigs []*rpc.TransactionSignature
	var beforeSig solana.Signature

	for page := 0; page < solanaMaxFetchSignaturesPages; page++ {
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
			return newSigs
		}

		// 如果返回数量小于查询限制，说明已获取所有签名，无需分页
		if len(signatures) < int(queryLimit) {
			return newSigs
		}

		// 需要继续分页，使用本批最旧的签名作为下次查询的 Before 参数
		beforeSig = signatures[len(signatures)-1].Signature
	}

	c.Logger.WithFields(logFields...).Infof("fetchNewSignatures reached max pages[%d], return %d sigs collected so far",
		solanaMaxFetchSignaturesPages, len(newSigs))
	return newSigs
}

// processTransactionSignatures 处理交易签名列表。
// 返回值：
//   - processedSlot: 已成功处理的最大 slot（仅在当前签名 **及之前所有签名** 都成功时才会推进）
//   - allOK: 本批签名是否全部成功处理
//
// 任一签名失败则立刻停止推进 processedSlot，避免跳过失败 slot 导致事件永久丢失。
func (c *SolanaClient) processTransactionSignatures(sigs []*rpc.TransactionSignature, currentSlot uint64,
	chainConfName, contractConfName, chainType, contractType string, logFields []logx.LogField) (uint64, bool) {

	processedSlot := currentSlot
	for _, txSig := range sigs {
		// 获取交易详情（通过可注入的 txFetcher，便于单测 mock）
		txResult, err := c.txFetcher(c.ctx, txSig.Signature, &rpc.GetTransactionOpts{
			Commitment: c.commitment,
		})
		if err != nil {
			c.Logger.WithFields(logFields...).Errorf("failed to get transaction %s: %v, stop advancing slot at %d",
				txSig.Signature.String(), err, processedSlot)
			return processedSlot, false
		}

		if txResult == nil {
			c.Logger.WithFields(logFields...).Errorf("got nil transaction %s, stop advancing slot at %d",
				txSig.Signature.String(), processedSlot)
			return processedSlot, false
		}

		// 解析交易日志并发布事件
		if pubErr := c.processAndPublishEvent(txResult, txSig.Signature.String(), txSig.Slot,
			chainConfName, contractConfName, chainType, contractType, logFields); pubErr != nil {
			c.Logger.WithFields(logFields...).Errorf("failed to publish event for %s: %v, stop advancing slot at %d",
				txSig.Signature.String(), pubErr, processedSlot)
			return processedSlot, false
		}

		// 记录已处理的最大 slot
		if txSig.Slot > processedSlot {
			processedSlot = txSig.Slot
		}
	}
	return processedSlot, true
}

// sortTransactionSignatures 按 slot 从小到大排序交易签名，确保事件按区块顺序处理
func sortTransactionSignatures(sigs []*rpc.TransactionSignature) {
	sort.Slice(sigs, func(i, j int) bool {
		return sigs[i].Slot < sigs[j].Slot
	})
}

// processAndPublishEvent 解析交易结果并发布事件到 Redis。返回非 nil 错误表示发布失败。
func (c *SolanaClient) processAndPublishEvent(txResult *rpc.GetTransactionResult, signature string, slot uint64,
	chainConfName, contractConfName, chainType, contractType string, logFields []logx.LogField) error {

	if txResult == nil || txResult.Meta == nil {
		// 交易无 meta 视作无事件可发布，直接返回 nil（不阻塞 slot 推进）
		return nil
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
		return err
	}

	// 发布事件到 Redis
	err = c.redisClient.PublishTradeGuardEventToStream(c.ctx, string(eventBytes), chainType,
		chainConfName, contractConfName, contractType,
		c.contractConfigs[contractConfName].ContractType)
	if err != nil {
		c.Logger.WithFields(logFields...).Errorf("failed to publish event to redis: %v", err)
		return err
	}

	c.Logger.WithFields(logFields...).Infof("published history event: signature=%s, slot=%d", signature, slot)
	return nil
}

// ptrInt 返回 int 的指针，用于 rpc.GetSignaturesForAddressOpts 的 Limit 字段
func ptrInt(v int) *int {
	return &v
}
