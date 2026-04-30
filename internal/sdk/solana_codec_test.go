package sdk

import (
	"encoding/binary"
	"encoding/hex"
	"testing"

	"github.com/gagliardetto/solana-go"
	"github.com/jackz-jones/blockchain-interactive-service/internal/config"
	pb "github.com/jackz-jones/blockchain-interactive-service/pb"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestParseDiscriminator 覆盖 discriminator 解析的正常与异常分支
func TestParseDiscriminator(t *testing.T) {
	cases := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{"valid 8 bytes hex", "e445a52e51cb9a1d", false},
		{"empty", "", true},
		{"non hex", "zz", true},
		{"too short", "e445a52e", true},
		{"too long", "e445a52e51cb9a1dff", true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := ParseDiscriminator(tc.input)
			if tc.wantErr {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Len(t, got, SolanaDiscriminatorSize)
		})
	}
}

// TestEncodeInstructionData_NoArgs 无参方法应仅返回 8 字节 discriminator
func TestEncodeInstructionData_NoArgs(t *testing.T) {
	spec := config.SolanaMethodSpec{Discriminator: "0102030405060708"}
	out, err := EncodeInstructionData(spec, nil)
	require.NoError(t, err)
	assert.Equal(t, []byte{1, 2, 3, 4, 5, 6, 7, 8}, out)
}

// TestEncodeInstructionData_MissingDiscriminator 缺失 discriminator 必须返回错误，不得退化为 JSON
func TestEncodeInstructionData_MissingDiscriminator(t *testing.T) {
	spec := config.SolanaMethodSpec{Discriminator: ""}
	_, err := EncodeInstructionData(spec, nil)
	assert.Error(t, err)
}

// TestEncodeInstructionData_AllBorshTypes 覆盖全部支持的 Borsh 类型
func TestEncodeInstructionData_AllBorshTypes(t *testing.T) {
	pk := solana.NewWallet().PublicKey()
	spec := config.SolanaMethodSpec{
		Discriminator: "0000000000000000",
		ArgSchema: []config.SolanaArgSpec{
			{Name: "a_u8", Type: "u8"},
			{Name: "a_u16", Type: "u16"},
			{Name: "a_u32", Type: "u32"},
			{Name: "a_u64", Type: "u64"},
			{Name: "a_i64", Type: "i64"},
			{Name: "a_bool", Type: "bool"},
			{Name: "a_str", Type: "string"},
			{Name: "a_pk", Type: "pubkey"},
			{Name: "a_bytes", Type: "bytes"},
		},
	}
	args := []*pb.KeyValuePair{
		{Key: "a_u8", Value: []byte("255")},
		{Key: "a_u16", Value: []byte("65535")},
		{Key: "a_u32", Value: []byte("4294967295")},
		{Key: "a_u64", Value: []byte("1")},
		{Key: "a_i64", Value: []byte("-1")},
		{Key: "a_bool", Value: []byte("true")},
		{Key: "a_str", Value: []byte("hi")},
		{Key: "a_pk", Value: []byte(pk.String())},
		{Key: "a_bytes", Value: []byte{0xAB, 0xCD}},
	}

	out, err := EncodeInstructionData(spec, args)
	require.NoError(t, err)

	// 前 8 字节是 discriminator
	assert.Equal(t, make([]byte, 8), out[:8])
	body := out[8:]

	// u8(255) -> 1 字节
	assert.Equal(t, byte(255), body[0])
	body = body[1:]

	// u16(65535) 小端
	assert.Equal(t, uint16(65535), binary.LittleEndian.Uint16(body[:2]))
	body = body[2:]

	// u32(4294967295) 小端
	assert.Equal(t, uint32(4294967295), binary.LittleEndian.Uint32(body[:4]))
	body = body[4:]

	// u64(1) 小端
	assert.Equal(t, uint64(1), binary.LittleEndian.Uint64(body[:8]))
	body = body[8:]

	// i64(-1) 小端，位模式为全 1
	assert.Equal(t, uint64(0xFFFFFFFFFFFFFFFF), binary.LittleEndian.Uint64(body[:8]))
	body = body[8:]

	// bool(true) -> 1
	assert.Equal(t, byte(1), body[0])
	body = body[1:]

	// string("hi") -> 4 字节长度 + "hi"
	assert.Equal(t, uint32(2), binary.LittleEndian.Uint32(body[:4]))
	assert.Equal(t, "hi", string(body[4:6]))
	body = body[6:]

	// pubkey -> 32 字节 raw
	assert.Equal(t, pk.Bytes(), body[:32])
	body = body[32:]

	// bytes(Vec<u8>) -> 4 字节长度 + 原始
	assert.Equal(t, uint32(2), binary.LittleEndian.Uint32(body[:4]))
	assert.Equal(t, []byte{0xAB, 0xCD}, body[4:6])
}

// TestEncodeInstructionData_MissingArg 缺失必填参数必须返回错误
func TestEncodeInstructionData_MissingArg(t *testing.T) {
	spec := config.SolanaMethodSpec{
		Discriminator: "0000000000000000",
		ArgSchema:     []config.SolanaArgSpec{{Name: "amount", Type: "u64"}},
	}
	_, err := EncodeInstructionData(spec, nil)
	assert.Error(t, err)
}

// TestEncodeInstructionData_InvalidU64 非法 u64 值必须返回参数错误
func TestEncodeInstructionData_InvalidU64(t *testing.T) {
	spec := config.SolanaMethodSpec{
		Discriminator: "0000000000000000",
		ArgSchema:     []config.SolanaArgSpec{{Name: "x", Type: "u64"}},
	}
	_, err := EncodeInstructionData(spec, []*pb.KeyValuePair{{Key: "x", Value: []byte("notanumber")}})
	assert.Error(t, err)
}

// TestEncodeInstructionData_UnsupportedType 未知类型应该报错
func TestEncodeInstructionData_UnsupportedType(t *testing.T) {
	spec := config.SolanaMethodSpec{
		Discriminator: "0000000000000000",
		ArgSchema:     []config.SolanaArgSpec{{Name: "x", Type: "unsupported"}},
	}
	_, err := EncodeInstructionData(spec, []*pb.KeyValuePair{{Key: "x", Value: []byte("1")}})
	assert.Error(t, err)
}

// TestBuildAccountMetaSlice_DefaultWhenEmpty 未配置 Accounts 时使用默认并置 usedDefault
func TestBuildAccountMetaSlice_DefaultWhenEmpty(t *testing.T) {
	from := solana.NewWallet().PublicKey()
	accs, usedDefault, err := BuildAccountMetaSlice(nil, from)
	require.NoError(t, err)
	assert.True(t, usedDefault)
	require.Len(t, accs, 1)
	assert.True(t, accs[0].PublicKey.Equals(from))
	assert.True(t, accs[0].IsSigner)
	assert.True(t, accs[0].IsWritable)
}

// TestBuildAccountMetaSlice_ResolvesFromAddressPlaceholder $fromAddress 占位符应被解析为 from
func TestBuildAccountMetaSlice_ResolvesFromAddressPlaceholder(t *testing.T) {
	from := solana.NewWallet().PublicKey()
	input := []config.SolanaAccountMeta{
		{Pubkey: "$fromAddress", IsSigner: true, IsWritable: true},
	}
	accs, usedDefault, err := BuildAccountMetaSlice(input, from)
	require.NoError(t, err)
	assert.False(t, usedDefault)
	require.Len(t, accs, 1)
	assert.True(t, accs[0].PublicKey.Equals(from))
}

// TestBuildAccountMetaSlice_SignerMismatch 声明了 signer 但 pubkey 与 fromAddress 不一致应返回错误
func TestBuildAccountMetaSlice_SignerMismatch(t *testing.T) {
	from := solana.NewWallet().PublicKey()
	other := solana.NewWallet().PublicKey()
	input := []config.SolanaAccountMeta{
		{Pubkey: other.String(), IsSigner: true, IsWritable: true},
	}
	_, _, err := BuildAccountMetaSlice(input, from)
	assert.Error(t, err)
}

// TestBuildAccountMetaSlice_InvalidPubkey 非法 pubkey 应返回错误
func TestBuildAccountMetaSlice_InvalidPubkey(t *testing.T) {
	from := solana.NewWallet().PublicKey()
	input := []config.SolanaAccountMeta{
		{Pubkey: "not_a_base58_pubkey", IsSigner: false, IsWritable: false},
	}
	_, _, err := BuildAccountMetaSlice(input, from)
	assert.Error(t, err)
}

// TestResolvePubkey_Normal 普通 base58 pubkey 应被解析
func TestResolvePubkey_Normal(t *testing.T) {
	from := solana.NewWallet().PublicKey()
	want := solana.NewWallet().PublicKey()
	got, err := resolvePubkey(want.String(), from)
	require.NoError(t, err)
	assert.True(t, got.Equals(want))
}

// ensureHexParses 工具：保证测试中写入的 hex discriminator 都合法
func ensureHexParses(t *testing.T, h string) {
	_, err := hex.DecodeString(h)
	require.NoErrorf(t, err, "hex %q should decode", h)
}

// TestDiscriminatorHexSamples 保证常用样例 discriminator 的 hex 是合法的
func TestDiscriminatorHexSamples(t *testing.T) {
	samples := []string{"0000000000000000", "e445a52e51cb9a1d", "0102030405060708"}
	for _, s := range samples {
		ensureHexParses(t, s)
	}
}
