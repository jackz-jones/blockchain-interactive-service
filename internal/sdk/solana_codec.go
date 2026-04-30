// Package sdk 提供多链交互的 SDK 封装。
// solana_codec.go 实现 Solana 指令数据的 Borsh 序列化与 Anchor discriminator 解析。
package sdk

import (
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"strconv"

	"github.com/gagliardetto/solana-go"
	"github.com/jackz-jones/blockchain-interactive-service/internal/config"
	pb "github.com/jackz-jones/blockchain-interactive-service/pb"
)

// Solana 参数类型枚举常量
const (
	SolanaArgTypeU8     = "u8"
	SolanaArgTypeU16    = "u16"
	SolanaArgTypeU32    = "u32"
	SolanaArgTypeU64    = "u64"
	SolanaArgTypeI64    = "i64"
	SolanaArgTypeBool   = "bool"
	SolanaArgTypeString = "string"
	SolanaArgTypePubkey = "pubkey"
	SolanaArgTypeBytes  = "bytes"

	// SolanaDiscriminatorSize Anchor/自定义方法判别符长度（字节）
	SolanaDiscriminatorSize = 8
)

// ParseDiscriminator 将 hex 字符串解析为 8 字节的方法判别符。
// 若 hex 非法或长度不为 8 字节，则返回明确错误。
func ParseDiscriminator(hexStr string) ([]byte, error) {
	if hexStr == "" {
		return nil, fmt.Errorf("discriminator is empty")
	}

	raw, err := hex.DecodeString(hexStr)
	if err != nil {
		return nil, fmt.Errorf("discriminator[%s] is not valid hex: %v", hexStr, err)
	}

	if len(raw) != SolanaDiscriminatorSize {
		return nil, fmt.Errorf("discriminator length must be %d bytes, got %d", SolanaDiscriminatorSize, len(raw))
	}

	return raw, nil
}

// kvPairToMap 将 []*pb.KeyValuePair 按 Key 索引为 map，便于按 ArgSchema 顺序取值。
func kvPairToMap(args []*pb.KeyValuePair) map[string][]byte {
	params := make(map[string][]byte, len(args))
	for _, a := range args {
		if a == nil {
			continue
		}
		params[a.Key] = a.Value
	}
	return params
}

// EncodeInstructionData 按 "8 字节 discriminator + Borsh(args)" 构造 Solana 指令数据。
// 入参：
//   - spec: 方法规范，必须包含 Discriminator；ArgSchema 可为空（表示无参方法）。
//   - args: 调用方传入的 KeyValuePair 列表，Key 对应 ArgSchema.Name。
//
// 注意：不得在 spec.Discriminator 缺失时退化为 JSON 编码，必须返回错误。
func EncodeInstructionData(spec config.SolanaMethodSpec, args []*pb.KeyValuePair) ([]byte, error) {
	disc, err := ParseDiscriminator(spec.Discriminator)
	if err != nil {
		return nil, fmt.Errorf("parse discriminator: %v", err)
	}

	out := make([]byte, 0, len(disc)+16)
	out = append(out, disc...)

	if len(spec.ArgSchema) == 0 {
		return out, nil
	}

	params := kvPairToMap(args)
	for _, argSpec := range spec.ArgSchema {
		raw, ok := params[argSpec.Name]
		if !ok {
			return nil, fmt.Errorf("argument[%s] not provided", argSpec.Name)
		}

		encoded, encErr := encodeBorshValue(argSpec.Type, raw)
		if encErr != nil {
			return nil, fmt.Errorf("encode argument[%s] type[%s]: %v", argSpec.Name, argSpec.Type, encErr)
		}
		out = append(out, encoded...)
	}

	return out, nil
}

// encodeBorshValue 将原始字节（通常来自 pb.KeyValuePair.Value，可视为该参数的字符串形式）
// 按 Borsh 规范编码为对应类型的小端序二进制。
func encodeBorshValue(argType string, raw []byte) ([]byte, error) {
	switch argType {
	case SolanaArgTypeU8:
		n, err := strconv.ParseUint(string(raw), 10, 8)
		if err != nil {
			return nil, fmt.Errorf("invalid u8 value: %v", err)
		}
		return []byte{byte(n)}, nil

	case SolanaArgTypeU16:
		n, err := strconv.ParseUint(string(raw), 10, 16)
		if err != nil {
			return nil, fmt.Errorf("invalid u16 value: %v", err)
		}
		buf := make([]byte, 2)
		binary.LittleEndian.PutUint16(buf, uint16(n))
		return buf, nil

	case SolanaArgTypeU32:
		n, err := strconv.ParseUint(string(raw), 10, 32)
		if err != nil {
			return nil, fmt.Errorf("invalid u32 value: %v", err)
		}
		buf := make([]byte, 4)
		binary.LittleEndian.PutUint32(buf, uint32(n))
		return buf, nil

	case SolanaArgTypeU64:
		n, err := strconv.ParseUint(string(raw), 10, 64)
		if err != nil {
			return nil, fmt.Errorf("invalid u64 value: %v", err)
		}
		buf := make([]byte, 8)
		binary.LittleEndian.PutUint64(buf, n)
		return buf, nil

	case SolanaArgTypeI64:
		n, err := strconv.ParseInt(string(raw), 10, 64)
		if err != nil {
			return nil, fmt.Errorf("invalid i64 value: %v", err)
		}
		buf := make([]byte, 8)
		binary.LittleEndian.PutUint64(buf, uint64(n))
		return buf, nil

	case SolanaArgTypeBool:
		switch string(raw) {
		case "true", "1":
			return []byte{1}, nil
		case "false", "0":
			return []byte{0}, nil
		default:
			return nil, fmt.Errorf("invalid bool value: %s", string(raw))
		}

	case SolanaArgTypeString:
		// Borsh string: 4 字节小端序长度 + UTF-8 字节
		s := string(raw)
		buf := make([]byte, 4+len(s))
		binary.LittleEndian.PutUint32(buf[:4], uint32(len(s)))
		copy(buf[4:], s)
		return buf, nil

	case SolanaArgTypePubkey:
		pk, err := solana.PublicKeyFromBase58(string(raw))
		if err != nil {
			return nil, fmt.Errorf("invalid pubkey value: %v", err)
		}
		out := make([]byte, 32)
		copy(out, pk.Bytes())
		return out, nil

	case SolanaArgTypeBytes:
		// Borsh Vec<u8>: 4 字节小端序长度 + 原始字节
		buf := make([]byte, 4+len(raw))
		binary.LittleEndian.PutUint32(buf[:4], uint32(len(raw)))
		copy(buf[4:], raw)
		return buf, nil

	default:
		return nil, fmt.Errorf("unsupported borsh type: %s", argType)
	}
}

// resolvePubkey 解析账户公钥：支持 "$fromAddress" 占位符与 base58 字符串。
func resolvePubkey(raw string, fromAddress solana.PublicKey) (solana.PublicKey, error) {
	if raw == "$fromAddress" {
		return fromAddress, nil
	}
	return solana.PublicKeyFromBase58(raw)
}

// BuildAccountMetaSlice 根据 MethodSpec.Accounts 构造 AccountMetaSlice。
// 若 spec.Accounts 为空，则返回默认 "[fromAddress(signer,writable)]" 并置 usedDefault=true
// 以便调用方打印 WARN 日志提醒。
func BuildAccountMetaSlice(accounts []config.SolanaAccountMeta, fromAddress solana.PublicKey) (
	solana.AccountMetaSlice, bool, error) {

	if len(accounts) == 0 {
		return solana.AccountMetaSlice{
			{PublicKey: fromAddress, IsSigner: true, IsWritable: true},
		}, true, nil
	}

	out := make(solana.AccountMetaSlice, 0, len(accounts))
	signerMatched := false
	for i, a := range accounts {
		pk, err := resolvePubkey(a.Pubkey, fromAddress)
		if err != nil {
			return nil, false, fmt.Errorf("account[%d] pubkey invalid: %v", i, err)
		}
		if a.IsSigner {
			// 当前实现只持有单一私钥（fromAddress）；若声明了 signer 但不是 fromAddress，
			// 说明本地私钥无法完成签名，必须返回明确错误。
			if !pk.Equals(fromAddress) {
				return nil, false, fmt.Errorf("account[%d] declared as signer but pubkey[%s] does not match local fromAddress[%s]",
					i, pk.String(), fromAddress.String())
			}
			signerMatched = true
		}
		out = append(out, &solana.AccountMeta{
			PublicKey:  pk,
			IsSigner:   a.IsSigner,
			IsWritable: a.IsWritable,
		})
	}

	// Solana 交易至少需要 feePayer 是 signer；若方法 Accounts 未显式声明 signer，
	// 则 feePayer（fromAddress）仍需作为隐式 signer 附加到 Transaction 中，由
	// solana.NewTransaction + TransactionPayer 处理，这里不强制报错。
	_ = signerMatched

	return out, false, nil
}
