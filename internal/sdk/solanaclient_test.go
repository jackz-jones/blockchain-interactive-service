package sdk

import (
	"context"
	"testing"
	"time"

	"github.com/mr-tron/base58"

	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/rpc"
	"github.com/jackz-jones/blockchain-interactive-service/internal/config"
	pb "github.com/jackz-jones/blockchain-interactive-service/pb"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBase58(t *testing.T) {
	sk := "2qsn2gEhmBk97rE2SsywnSrnXeyu1WZWd1RA5t8P1ERhVY3QJuEkPQoLzjxgvwZ9nX1nQjtnW6U7XZUaZB9rmer"
	ori, err := base58.Decode(sk)
	assert.NoError(t, err)

	encoded := base58.Encode(ori)
	assert.NotEmpty(t, encoded)

	t.Logf("base58 encoded sk: %s", encoded)
}

// getTestSolanaConf 返回用于测试的 Solana 配置，使用 QuickNode 公共 Devnet 节点
// 注意：官方的 api.devnet.solana.com 可能无法访问，这里使用 QuickNode 提供的公共 Devnet 节点
func getTestSolanaConf() config.SolanaConf {
	return config.SolanaConf{
		RpcUrl: "https://docs-demo.solana-devnet.quiknode.pro/",
		WsUrl:  "wss://docs-demo.solana-devnet.quiknode.pro/",

		// 对应 Solana 地址：5pVyoAeURQHNMVU7DmfMHvCDNmTEYXWfEwc136GYhTKG，开发网公开的一个地址，账户余额管够
		PrivateKey: "5MaiiCavjCmn9Hs1o3eznqDEhRwxo7pXiAYez7keQUviUkauRiTMD8DrESdrNjN8zd9mTmVhRvBJeg5vhyvgrAhG",

		// 对应 Solana 地址：HVMZrfwBwH5aUm8kzXTPkF1kFcd2yZxwRNBU3HmqCBgv，phantom 钱包的地址，可以去水龙头充值：https://faucet.solana.com
		//PrivateKey:      "2qsn2gEhmBk97rE2SsywnSrnXeyu1WZWd1RA5t8P1ERhVY3QJuEkPQoLzjxgvwZ9nX1nQjtnW6U7XZUaZB9rmer",
		CommitmentLevel: "confirmed",
		SkipPreflight:   false,
		MaxRetries:      3,
	}
}

// TestNewSolanaClient 测试创建 SolanaClient 对象
func TestNewSolanaClient(t *testing.T) {
	tests := []struct {
		name        string
		solanaConf  config.SolanaConf
		expectError bool
	}{
		{
			name:        "valid devnet config",
			solanaConf:  getTestSolanaConf(),
			expectError: false,
		},
		{
			name: "invalid private key",
			solanaConf: config.SolanaConf{
				RpcUrl:          "https://docs-demo.solana-devnet.quiknode.pro/",
				WsUrl:           "wss://docs-demo.solana-devnet.quiknode.pro/",
				PrivateKey:      "invalid_private_key",
				CommitmentLevel: "confirmed",
				SkipPreflight:   false,
				MaxRetries:      3,
			},
			expectError: true,
		},
		{
			name: "empty rpc url",
			solanaConf: config.SolanaConf{
				RpcUrl:          "",
				WsUrl:           "wss://docs-demo.solana-devnet.quiknode.pro/",
				PrivateKey:      "5MaiiCavjCmn9Hs1o3eznqDEhRwxo7pXiAYez7keQUviUkauRiTMD8DrESdrNjN8zd9mTmVhRvBJeg5vhyvgrAhG",
				CommitmentLevel: "confirmed",
				SkipPreflight:   false,
				MaxRetries:      3,
			},
			expectError: true,
		},
		{
			name: "invalid commitment level defaults to confirmed",
			solanaConf: config.SolanaConf{
				RpcUrl:          "https://docs-demo.solana-devnet.quiknode.pro/",
				WsUrl:           "wss://docs-demo.solana-devnet.quiknode.pro/",
				PrivateKey:      "5MaiiCavjCmn9Hs1o3eznqDEhRwxo7pXiAYez7keQUviUkauRiTMD8DrESdrNjN8zd9mTmVhRvBJeg5vhyvgrAhG",
				CommitmentLevel: "invalid_level",
				SkipPreflight:   false,
				MaxRetries:      3,
			},
			expectError: false, // 无效的 commitment level 应该默认为 confirmed
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, err := NewSolanaClient(context.Background(), tt.solanaConf, nil, nil)

			if tt.expectError {
				assert.Error(t, err, "应该返回错误")
				assert.Nil(t, client, "出错时 client 应为 nil")
			} else {
				assert.NoError(t, err, "不应该返回错误")
				assert.NotNil(t, client, "client 不应为 nil")
				assert.Equal(t, tt.solanaConf.SkipPreflight, client.skipPreflight, "skipPreflight 应该正确设置")
				assert.Equal(t, tt.solanaConf.MaxRetries, client.maxRetries, "maxRetries 应该正确设置")

				// 清理
				_ = client.Stop()
			}
		})
	}
}

// TestNewSolanaClient_ConnectDevnet 测试连接 Solana Devnet 测试网
func TestNewSolanaClient_ConnectDevnet(t *testing.T) {
	if testing.Short() {
		t.Skip("跳过需要网络的测试")
	}

	solanaConf := getTestSolanaConf()
	client, err := NewSolanaClient(context.Background(), solanaConf, nil, nil)
	require.NoError(t, err, "连接 Devnet 不应该返回错误")
	require.NotNil(t, client, "client 不应为 nil")
	defer client.Stop()

	// 测试 GetHealth
	t.Run("GetHealth", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		health, err := client.rpcClient.GetHealth(ctx)
		if err != nil {
			t.Logf("GetHealth 返回错误（可能是 Devnet 节点不可用）: %v", err)
		} else {
			assert.Equal(t, rpc.HealthOk, health, "健康的节点应该返回 'ok'")
			t.Logf("Devnet 节点健康状态: %s", health)
		}
	})

	// 测试 GetLatestBlockhash
	t.Run("GetLatestBlockhash", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		result, err := client.rpcClient.GetLatestBlockhash(ctx, rpc.CommitmentConfirmed)
		if err != nil {
			t.Logf("GetLatestBlockhash 返回错误（可能是 Devnet 节点不可用）: %v", err)
		} else {
			assert.NotNil(t, result, "GetLatestBlockhash 结果不应为 nil")
			assert.NotNil(t, result.Value, "GetLatestBlockhash 的 Value 不应为 nil")
			assert.NotEmpty(t, result.Value.Blockhash.String(), "Blockhash 不应为空")
			t.Logf("获取到 Devnet 最新区块哈希: %s", result.Value.Blockhash.String())
		}
	})

	// 测试 GetSlot
	t.Run("GetSlot", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		slot, err := client.rpcClient.GetSlot(ctx, rpc.CommitmentConfirmed)
		if err != nil {
			t.Logf("GetSlot 返回错误（可能是 Devnet 节点不可用）: %v", err)
		} else {
			assert.Greater(t, slot, uint64(0), "Slot 应该大于 0")
			t.Logf("当前 Devnet slot: %d", slot)
		}
	})
}

// TestSolanaClient_GetTxByTxId_InvalidSignature 测试无效签名的错误处理
func TestSolanaClient_GetTxByTxId_InvalidSignature(t *testing.T) {
	solanaConf := getTestSolanaConf()
	client, err := NewSolanaClient(context.Background(), solanaConf, nil, nil)
	if err != nil {
		t.Skipf("无法创建 Solana 客户端: %v", err)
	}
	defer client.Stop()

	tests := []struct {
		name        string
		txId        string
		expectError bool
	}{
		{
			name:        "invalid base58 signature",
			txId:        "invalid_signature",
			expectError: true,
		},
		{
			name:        "valid signature format but not found",
			txId:        solana.Signature{}.String(), // 全零签名
			expectError: true,
		},
		{
			name:        "empty signature",
			txId:        "",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, _, err := client.GetTxByTxId(tt.txId)
			if tt.expectError {
				assert.Error(t, err, "应该返回错误")
			}
		})
	}
}

// TestSolanaClient_SendTransaction_UnknownContract 测试发送交易时使用未知合约配置
func TestSolanaClient_SendTransaction_UnknownContract(t *testing.T) {
	solanaConf := getTestSolanaConf()
	client, err := NewSolanaClient(context.Background(), solanaConf, nil, nil)
	if err != nil {
		t.Skipf("无法创建 Solana 客户端: %v", err)
	}
	defer client.Stop()

	_, _, err = client.SendTransaction(pb.MethodType_Invoke, "unknown_contract", "method", nil, 30, true)
	assert.Error(t, err, "使用未知合约配置名应该返回错误")
	assert.Contains(t, err.Error(), "unknown contract config name", "错误信息应包含 'unknown contract config name'")
}

// TestSolanaClient_Stop 测试停止客户端
func TestSolanaClient_Stop(t *testing.T) {
	solanaConf := getTestSolanaConf()
	client, err := NewSolanaClient(context.Background(), solanaConf, nil, nil)
	if err != nil {
		t.Skipf("无法创建 Solana 客户端: %v", err)
	}

	err = client.Stop()
	assert.NoError(t, err, "Stop 不应该返回错误")

	// 多次停止也不应该出错
	err = client.Stop()
	assert.NoError(t, err, "多次 Stop 也不应该返回错误")
}

// TestSolanaClient_CommitmentLevels 测试不同的确认级别
func TestSolanaClient_CommitmentLevels(t *testing.T) {
	tests := []struct {
		name               string
		commitmentLevel    string
		expectedCommitment rpc.CommitmentType
	}{
		{
			name:               "processed",
			commitmentLevel:    "processed",
			expectedCommitment: rpc.CommitmentProcessed,
		},
		{
			name:               "confirmed",
			commitmentLevel:    "confirmed",
			expectedCommitment: rpc.CommitmentConfirmed,
		},
		{
			name:               "finalized",
			commitmentLevel:    "finalized",
			expectedCommitment: rpc.CommitmentFinalized,
		},
		{
			name:               "invalid defaults to confirmed",
			commitmentLevel:    "invalid",
			expectedCommitment: rpc.CommitmentConfirmed,
		},
		{
			name:               "empty defaults to confirmed",
			commitmentLevel:    "",
			expectedCommitment: rpc.CommitmentConfirmed,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			solanaConf := config.SolanaConf{
				RpcUrl:          "https://docs-demo.solana-devnet.quiknode.pro/",
				WsUrl:           "wss://docs-demo.solana-devnet.quiknode.pro/",
				PrivateKey:      "5MaiiCavjCmn9Hs1o3eznqDEhRwxo7pXiAYez7keQUviUkauRiTMD8DrESdrNjN8zd9mTmVhRvBJeg5vhyvgrAhG",
				CommitmentLevel: tt.commitmentLevel,
				SkipPreflight:   false,
				MaxRetries:      3,
			}

			client, err := NewSolanaClient(context.Background(), solanaConf, nil, nil)
			assert.NoError(t, err, "创建客户端不应该返回错误")
			assert.Equal(t, tt.expectedCommitment, client.commitment, "commitment 应该正确设置")
			_ = client.Stop()
		})
	}
}

// TestSolanaClient_GetVersion 测试获取节点版本信息
func TestSolanaClient_GetVersion(t *testing.T) {
	if testing.Short() {
		t.Skip("跳过需要网络的测试")
	}

	solanaConf := getTestSolanaConf()
	client, err := NewSolanaClient(context.Background(), solanaConf, nil, nil)
	if err != nil {
		t.Skipf("无法创建 Solana 客户端: %v", err)
	}
	defer client.Stop()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	version, err := client.rpcClient.GetVersion(ctx)
	if err != nil {
		t.Logf("GetVersion 返回错误（可能是 Devnet 节点不可用）: %v", err)
	} else {
		assert.NotEmpty(t, version, "版本信息不应为空")
		t.Logf("Devnet 节点版本: %v", version)
	}
}

// TestSolanaClient_GetGenesisHash 测试获取创世区块哈希
func TestSolanaClient_GetGenesisHash(t *testing.T) {
	if testing.Short() {
		t.Skip("跳过需要网络的测试")
	}

	solanaConf := getTestSolanaConf()
	client, err := NewSolanaClient(context.Background(), solanaConf, nil, nil)
	if err != nil {
		t.Skipf("无法创建 Solana 客户端: %v", err)
	}
	defer client.Stop()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	genesisHash, err := client.rpcClient.GetGenesisHash(ctx)
	if err != nil {
		t.Logf("GetGenesisHash 返回错误（可能是 Devnet 节点不可用）: %v", err)
	} else {
		assert.NotEmpty(t, genesisHash.String(), "创世哈希不应为空")
		t.Logf("Devnet 创世哈希: %s", genesisHash.String())
	}
}

// TestSolanaClient_GetBalance 测试获取账户余额
func TestSolanaClient_GetBalance(t *testing.T) {
	if testing.Short() {
		t.Skip("跳过需要网络的测试")
	}

	solanaConf := getTestSolanaConf()
	client, err := NewSolanaClient(context.Background(), solanaConf, nil, nil)
	if err != nil {
		t.Skipf("无法创建 Solana 客户端: %v", err)
	}
	defer client.Stop()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	result, err := client.rpcClient.GetBalance(ctx, client.fromAddress, rpc.CommitmentConfirmed)
	if err != nil {
		t.Logf("GetBalance 返回错误（可能是 Devnet 节点不可用）: %v", err)
	} else {
		assert.NotNil(t, result, "余额结果不应为 nil")
		assert.NotNil(t, result.Value, "余额 Value 不应为 nil")

		// Solana 的最小单位是 Lamport‌。
		// 1 SOL = 1,000,000,000 Lamports‌（即十亿个 Lamport)
		t.Logf("地址 %s 余额: %d Lamport", client.fromAddress.String(), result.Value)
	}
}

// TestSolanaClient_GetAccountInfo 测试获取账户信息
func TestSolanaClient_GetAccountInfo(t *testing.T) {
	if testing.Short() {
		t.Skip("跳过需要网络的测试")
	}

	solanaConf := getTestSolanaConf()
	client, err := NewSolanaClient(context.Background(), solanaConf, nil, nil)
	if err != nil {
		t.Skipf("无法创建 Solana 客户端: %v", err)
	}
	defer client.Stop()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	result, err := client.rpcClient.GetAccountInfo(ctx, client.fromAddress)
	if err != nil {
		t.Logf("GetAccountInfo 返回错误（可能是 Devnet 节点不可用）: %v", err)
	} else {
		assert.NotNil(t, result, "账户信息结果不应为 nil")
		if result.Value != nil {
			t.Logf("地址 %s 账户信息: owner=%s, lamports=%d, executable=%v",
				client.fromAddress.String(),
				result.Value.Owner.String(),
				result.Value.Lamports,
				result.Value.Executable)
		} else {
			t.Logf("地址 %s 账户不存在（Value 为 nil）", client.fromAddress.String())
		}
	}
}

// TestSolanaClient_SubscribeContractEvent_EmptyContractAddr 测试合约地址为空时的错误处理
func TestSolanaClient_SubscribeContractEvent_EmptyContractAddr(t *testing.T) {
	solanaConf := getTestSolanaConf()
	client, err := NewSolanaClient(context.Background(), solanaConf, nil, nil)
	if err != nil {
		t.Skipf("无法创建 Solana 客户端: %v", err)
	}
	defer client.Stop()

	// 测试空的合约地址
	contractConf := config.ContractConf{
		ContractAddr: "",
	}

	err = client.SubscribeContractEvent(contractConf, "testChain", "testContract", "solana", "notification")
	assert.Error(t, err, "空的合约地址应该返回错误")
	assert.Contains(t, err.Error(), "empty", "错误信息应包含 'empty'")
}

// TestSolanaClient_SubscribeContractEvent_InvalidContractAddr 测试无效合约地址的错误处理
func TestSolanaClient_SubscribeContractEvent_InvalidContractAddr(t *testing.T) {
	solanaConf := getTestSolanaConf()
	client, err := NewSolanaClient(context.Background(), solanaConf, nil, nil)
	if err != nil {
		t.Skipf("无法创建 Solana 客户端: %v", err)
	}
	defer client.Stop()

	// 测试无效的合约地址
	contractConf := config.ContractConf{
		ContractAddr: "invalid_contract_address",
	}

	err = client.SubscribeContractEvent(contractConf, "testChain", "testContract", "solana", "notification")
	assert.Error(t, err, "无效的合约地址应该返回错误")
	assert.Contains(t, err.Error(), "invalid contract address", "错误信息应包含 'invalid contract address'")
}

// TestSolanaClient_SendTransaction_QueryMethod 测试查询类型的交易
func TestSolanaClient_SendTransaction_QueryMethod(t *testing.T) {
	if testing.Short() {
		t.Skip("跳过需要网络的测试")
	}

	solanaConf := getTestSolanaConf()
	contractConfs := map[string]*config.ContractConf{
		"test_contract": {
			EnableSubscribe:   false,
			ContractType:      "notification",
			ContractAddr:      "11111111111111111111111111111111", // 系统程序地址
			DeployBlockHeight: 0,
		},
	}

	client, err := NewSolanaClient(context.Background(), solanaConf, contractConfs, nil)
	if err != nil {
		t.Skipf("无法创建 Solana 客户端: %v", err)
	}
	defer client.Stop()

	// 查询类型的交易应该使用 SimulateTransaction 来查询状态
	_, _, err = client.SendTransaction(pb.MethodType_Query, "test_contract", "method", nil, 30, false)
	// 查询类型可能会因为模拟交易失败而返回错误，这是预期的
	t.Logf("SendTransaction 查询类型返回: err=%v", err)
}

// TestSolanaClient_ContextCancellation 测试上下文取消
func TestSolanaClient_ContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())

	solanaConf := getTestSolanaConf()
	client, err := NewSolanaClient(ctx, solanaConf, nil, nil)
	if err != nil {
		t.Skipf("无法创建 Solana 客户端: %v", err)
	}

	// 取消上下文
	cancel()

	// 在上下文取消后，客户端的操作应该返回上下文错误
	_, _, err = client.GetTxByTxId("1111111111111111111111111111111111111111111111111111111111111111")
	assert.Error(t, err, "上下文取消后操作应该返回错误")
	assert.Contains(t, err.Error(), "context", "错误应该与上下文相关")

	_ = client.Stop()
}

// TestSolanaClient_RequestAirdrop 测试在 Devnet 上请求空投
func TestSolanaClient_RequestAirdrop(t *testing.T) {
	if testing.Short() {
		t.Skip("跳过需要网络的测试")
	}

	solanaConf := getTestSolanaConf()
	client, err := NewSolanaClient(context.Background(), solanaConf, nil, nil)
	if err != nil {
		t.Skipf("无法创建 Solana 客户端: %v", err)
	}
	defer client.Stop()

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	// 在 Devnet 上请求 1 SOL 空投 (1 SOL = 1_000_000_000 lamports)
	signature, err := client.rpcClient.RequestAirdrop(ctx, client.fromAddress, 1_000_000_000, rpc.CommitmentConfirmed)
	if err != nil {
		t.Logf("RequestAirdrop 返回错误（可能是 Devnet 节点不可用）: %v", err)
	} else {
		assert.NotNil(t, signature, "空投签名不应为 nil")
		t.Logf("空投请求签名: %s", signature.String())
	}
}
