package sdk

import (
	"context"
	"errors"
	"testing"

	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/rpc"
	"github.com/stretchr/testify/assert"
	"github.com/zeromicro/go-zero/core/logx"
)

// newTestSolanaClientForProcess 构造一个最小可用的 SolanaClient 实例用于纯逻辑测试。
// 不创建真实 RPC 连接，避免单测对外部网络的依赖。
func newTestSolanaClientForProcess(fetcher func(ctx context.Context, sig solana.Signature,
	opts *rpc.GetTransactionOpts) (*rpc.GetTransactionResult, error)) *SolanaClient {
	ctx, cancel := context.WithCancel(context.Background())
	return &SolanaClient{
		ctx:        ctx,
		cancel:     cancel,
		commitment: rpc.CommitmentConfirmed,
		Logger:     logx.WithContext(ctx),
		txFetcher:  fetcher,
	}
}

// mkSig 生成一个可用于测试的签名占位值
func mkSig(b byte) solana.Signature {
	var s solana.Signature
	s[0] = b
	return s
}

// TestSortTransactionSignatures_AscendingBySlot 验证签名按 slot 升序排序
func TestSortTransactionSignatures_AscendingBySlot(t *testing.T) {
	sigs := []*rpc.TransactionSignature{
		{Signature: mkSig(3), Slot: 30},
		{Signature: mkSig(1), Slot: 10},
		{Signature: mkSig(2), Slot: 20},
	}
	sortTransactionSignatures(sigs)
	assert.Equal(t, uint64(10), sigs[0].Slot)
	assert.Equal(t, uint64(20), sigs[1].Slot)
	assert.Equal(t, uint64(30), sigs[2].Slot)
}

// TestProcessTransactionSignatures_FirstFailureDoesNotAdvanceSlot 验证核心语义：
// 第一笔签名获取失败时，processedSlot 保持在 currentSlot，且返回 allOK=false，
// 从而上层不会推进 currentSlot，保证失败 slot 在下一轮可被重试。
func TestProcessTransactionSignatures_FirstFailureDoesNotAdvanceSlot(t *testing.T) {
	fetcher := func(ctx context.Context, sig solana.Signature,
		opts *rpc.GetTransactionOpts) (*rpc.GetTransactionResult, error) {
		return nil, errors.New("mock fetch failed")
	}
	c := newTestSolanaClientForProcess(fetcher)
	defer c.cancel()

	sigs := []*rpc.TransactionSignature{
		{Signature: mkSig(1), Slot: 100},
		{Signature: mkSig(2), Slot: 101},
	}
	processed, allOK := c.processTransactionSignatures(sigs, 99,
		"chain", "contract", "solana", "notify", nil)
	assert.Equal(t, uint64(99), processed, "失败不应推进 processedSlot")
	assert.False(t, allOK, "整批应标记为未全部成功")
}

// TestProcessTransactionSignatures_NilResultTreatedAsFailure 获取到 nil 结果也视为失败，不推进 slot
func TestProcessTransactionSignatures_NilResultTreatedAsFailure(t *testing.T) {
	fetcher := func(ctx context.Context, sig solana.Signature,
		opts *rpc.GetTransactionOpts) (*rpc.GetTransactionResult, error) {
		return nil, nil
	}
	c := newTestSolanaClientForProcess(fetcher)
	defer c.cancel()

	sigs := []*rpc.TransactionSignature{
		{Signature: mkSig(1), Slot: 100},
	}
	processed, allOK := c.processTransactionSignatures(sigs, 99,
		"chain", "contract", "solana", "notify", nil)
	assert.Equal(t, uint64(99), processed)
	assert.False(t, allOK)
}

// TestProcessTransactionSignatures_EmptyInput 空输入返回 currentSlot 且 allOK=true
func TestProcessTransactionSignatures_EmptyInput(t *testing.T) {
	c := newTestSolanaClientForProcess(nil)
	defer c.cancel()

	processed, allOK := c.processTransactionSignatures(nil, 42,
		"chain", "contract", "solana", "notify", nil)
	assert.Equal(t, uint64(42), processed)
	assert.True(t, allOK)
}

// TestPtrInt 简单覆盖 helper
func TestPtrInt(t *testing.T) {
	p := ptrInt(7)
	assert.NotNil(t, p)
	assert.Equal(t, 7, *p)
}
