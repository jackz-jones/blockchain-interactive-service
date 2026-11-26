package validator

import (
	"testing"

	"github.com/jackz-jones/blockchain-interactive-service/internal/store"
)

// 提取的重复字符串常量（goconst）
const (
	testEthLocalURL    = "http://localhost:8545"
	testSolanaLocalURL = "http://localhost:8899"

	abcdefghijklmnop1234567890Str = "abcdefghijklmnop1234567890"
	permissionedwithcertStr       = "permissionedwithcert"
	s12700112301                  = "127.0.0.1:12301"
	base64signkeyStr              = "base64signkey"
	chainmakerStr                 = "chainmaker"
	base64certStr                 = "base64cert"
	ethereumStr                   = "ethereum"
	solanaStr                     = "solana"
	publicStr                     = "public"
	chain1Str                     = "chain1"
	org1Str                       = "org1"
	SM3Str                        = "SM3"
)

// ========== ValidateChainConfig 测试 ==========

func TestValidateChainConfig_ChainMaker_ValidPublicMode(t *testing.T) {
	req := &CreateChainConfigRequestBody{
		ChainType: chainmakerStr,
		ChainId:   chain1Str,
		AuthType:  publicStr,
		HashType:  SHA256Str,
		SignKey:   base64signkeyStr,
		Nodes: []NodeRequest{
			{NodeAddr: s12700112301},
		},
	}

	if err := ValidateChainConfig(req); err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
}

func TestValidateChainConfig_ChainMaker_ValidCertMode(t *testing.T) {
	req := &CreateChainConfigRequestBody{
		ChainType: chainmakerStr,
		ChainId:   chain1Str,
		AuthType:  permissionedwithcertStr,
		OrgId:     org1Str,
		HashType:  SM3Str,
		SignKey:   base64signkeyStr,
		SignCert:  base64certStr,
		Nodes: []NodeRequest{
			{NodeAddr: s12700112301},
		},
	}

	if err := ValidateChainConfig(req); err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
}

func TestValidateChainConfig_ChainMaker_MissingChainId(t *testing.T) {
	req := &CreateChainConfigRequestBody{
		ChainType: chainmakerStr,
		AuthType:  publicStr,
		HashType:  SHA256Str,
		SignKey:   base64signkeyStr,
		Nodes: []NodeRequest{
			{NodeAddr: s12700112301},
		},
	}

	err := ValidateChainConfig(req)
	if err == nil {
		t.Fatal("expected error for missing chain_id")
	}
}

func TestValidateChainConfig_ChainMaker_MissingAuthType(t *testing.T) {
	req := &CreateChainConfigRequestBody{
		ChainType: chainmakerStr,
		ChainId:   chain1Str,
		HashType:  SHA256Str,
		SignKey:   base64signkeyStr,
		Nodes: []NodeRequest{
			{NodeAddr: s12700112301},
		},
	}

	err := ValidateChainConfig(req)
	if err == nil {
		t.Fatal("expected error for missing auth_type")
	}
}

func TestValidateChainConfig_ChainMaker_InvalidAuthType(t *testing.T) {
	req := &CreateChainConfigRequestBody{
		ChainType: chainmakerStr,
		ChainId:   chain1Str,
		AuthType:  "invalid_mode",
		HashType:  SHA256Str,
		SignKey:   base64signkeyStr,
		Nodes: []NodeRequest{
			{NodeAddr: s12700112301},
		},
	}

	err := ValidateChainConfig(req)
	if err == nil {
		t.Fatal("expected error for invalid auth_type")
	}
}

func TestValidateChainConfig_ChainMaker_MissingHashType(t *testing.T) {
	req := &CreateChainConfigRequestBody{
		ChainType: chainmakerStr,
		ChainId:   chain1Str,
		AuthType:  publicStr,
		SignKey:   base64signkeyStr,
		Nodes: []NodeRequest{
			{NodeAddr: s12700112301},
		},
	}

	err := ValidateChainConfig(req)
	if err == nil {
		t.Fatal("expected error for missing hash_type")
	}
}

func TestValidateChainConfig_ChainMaker_InvalidHashType(t *testing.T) {
	req := &CreateChainConfigRequestBody{
		ChainType: chainmakerStr,
		ChainId:   chain1Str,
		AuthType:  publicStr,
		HashType:  "MD5",
		SignKey:   base64signkeyStr,
		Nodes: []NodeRequest{
			{NodeAddr: s12700112301},
		},
	}

	err := ValidateChainConfig(req)
	if err == nil {
		t.Fatal("expected error for invalid hash_type")
	}
}

func TestValidateChainConfig_ChainMaker_EmptyNodes(t *testing.T) {
	req := &CreateChainConfigRequestBody{
		ChainType: chainmakerStr,
		ChainId:   chain1Str,
		AuthType:  publicStr,
		HashType:  SHA256Str,
		SignKey:   base64signkeyStr,
		Nodes:     []NodeRequest{},
	}

	err := ValidateChainConfig(req)
	if err == nil {
		t.Fatal("expected error for empty nodes")
	}
}

func TestValidateChainConfig_ChainMaker_NodeMissingAddr(t *testing.T) {
	req := &CreateChainConfigRequestBody{
		ChainType: chainmakerStr,
		ChainId:   chain1Str,
		AuthType:  publicStr,
		HashType:  SHA256Str,
		SignKey:   base64signkeyStr,
		Nodes: []NodeRequest{
			{NodeAddr: ""},
		},
	}

	err := ValidateChainConfig(req)
	if err == nil {
		t.Fatal("expected error for node missing node_addr")
	}
}

func TestValidateChainConfig_ChainMaker_CertModeMissingOrgId(t *testing.T) {
	req := &CreateChainConfigRequestBody{
		ChainType: chainmakerStr,
		ChainId:   chain1Str,
		AuthType:  permissionedwithcertStr,
		HashType:  SHA256Str,
		SignKey:   base64signkeyStr,
		SignCert:  base64certStr,
		Nodes: []NodeRequest{
			{NodeAddr: s12700112301},
		},
	}

	err := ValidateChainConfig(req)
	if err == nil {
		t.Fatal("expected error for missing org_id in cert mode")
	}
}

func TestValidateChainConfig_ChainMaker_CertModeMissingSignCert(t *testing.T) {
	req := &CreateChainConfigRequestBody{
		ChainType: chainmakerStr,
		ChainId:   chain1Str,
		AuthType:  permissionedwithcertStr,
		OrgId:     org1Str,
		HashType:  SHA256Str,
		SignKey:   base64signkeyStr,
		Nodes: []NodeRequest{
			{NodeAddr: s12700112301},
		},
	}

	err := ValidateChainConfig(req)
	if err == nil {
		t.Fatal("expected error for missing sign_cert in cert mode")
	}
}

func TestValidateChainConfig_ChainMaker_MissingSignKey(t *testing.T) {
	req := &CreateChainConfigRequestBody{
		ChainType: chainmakerStr,
		ChainId:   chain1Str,
		AuthType:  publicStr,
		HashType:  SHA256Str,
		Nodes: []NodeRequest{
			{NodeAddr: s12700112301},
		},
	}

	err := ValidateChainConfig(req)
	if err == nil {
		t.Fatal("expected error for missing sign_key")
	}
}

func TestValidateChainConfig_Ethereum_Valid(t *testing.T) {
	req := &CreateChainConfigRequestBody{
		ChainType: ethereumStr,
		HttpUrl:   testEthLocalURL,
	}

	if err := ValidateChainConfig(req); err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
}

func TestValidateChainConfig_Ethereum_MissingHttpUrl(t *testing.T) {
	req := &CreateChainConfigRequestBody{
		ChainType: ethereumStr,
	}

	err := ValidateChainConfig(req)
	if err == nil {
		t.Fatal("expected error for missing http_url")
	}
}

func TestValidateChainConfig_Solana_Valid(t *testing.T) {
	req := &CreateChainConfigRequestBody{
		ChainType: solanaStr,
		SolRpcUrl: testSolanaLocalURL,
	}

	if err := ValidateChainConfig(req); err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
}

func TestValidateChainConfig_Solana_MissingRpcUrl(t *testing.T) {
	req := &CreateChainConfigRequestBody{
		ChainType: solanaStr,
	}

	err := ValidateChainConfig(req)
	if err == nil {
		t.Fatal("expected error for missing sol_rpc_url")
	}
}

// ========== MaskChainConfigSensitiveFields 测试 ==========

func TestMaskChainConfigSensitiveFields_ChainMaker(t *testing.T) {
	config := &store.TenantChainConfig{
		ChainType:  chainmakerStr,
		SignKey:    abcdefghijklmnop1234567890Str,
		UserTlsKey: "tlskey1234567890abcdef",
		UserEncKey: "enckey1234567890abcdef",
		ChainId:    chain1Str,
	}

	masked := MaskChainConfigSensitiveFields(config)

	// 原始对象不应被修改
	if config.SignKey != abcdefghijklmnop1234567890Str {
		t.Errorf("original SignKey should not be modified, got: %s", config.SignKey)
	}

	// 私钥不出域：副本中敏感字段为掩码（前 4 + **** + 后 4），非明文也非空
	if masked.SignKey != "abcd****7890" {
		t.Errorf("SignKey should be masked (first4+****+last4), got: %s", masked.SignKey)
	}
	if masked.UserTlsKey != "tlsk****cdef" {
		t.Errorf("UserTlsKey should be masked, got: %s", masked.UserTlsKey)
	}
	if masked.UserEncKey != "enck****cdef" {
		t.Errorf("UserEncKey should be masked, got: %s", masked.UserEncKey)
	}
	// 非敏感字段保持不变
	if masked.ChainId != chain1Str {
		t.Errorf("ChainId should not be affected, got: %s", masked.ChainId)
	}
}

func TestMaskChainConfigSensitiveFields_Ethereum(t *testing.T) {
	config := &store.TenantChainConfig{
		ChainType:  ethereumStr,
		PrivateKey: "0x1234567890abcdef1234567890abcdef",
		HttpUrl:    testEthLocalURL,
	}

	masked := MaskChainConfigSensitiveFields(config)

	if masked.PrivateKey != "0x12****cdef" {
		t.Errorf("PrivateKey should be masked (first4+****+last4), got: %s", masked.PrivateKey)
	}
	if masked.HttpUrl != testEthLocalURL {
		t.Errorf("HttpUrl should not be affected, got: %s", masked.HttpUrl)
	}
}

func TestMaskChainConfigSensitiveFields_Solana(t *testing.T) {
	config := &store.TenantChainConfig{
		ChainType:     solanaStr,
		SolPrivateKey: "5KQwrPbwdL6PhXujxW37FSSQZ1JiwsST4cqQzDeyXtP79zkvFD3",
		SolRpcUrl:     testSolanaLocalURL,
	}

	masked := MaskChainConfigSensitiveFields(config)

	if masked.SolPrivateKey != "5KQw****vFD3" {
		t.Errorf("SolPrivateKey should be masked (first4+****+last4), got: %s", masked.SolPrivateKey)
	}
	if masked.SolRpcUrl != testSolanaLocalURL {
		t.Errorf("SolRpcUrl should not be affected, got: %s", masked.SolRpcUrl)
	}
}

func TestMaskChainConfigSensitiveFields_ShortKey(t *testing.T) {
	config := &store.TenantChainConfig{
		ChainType: chainmakerStr,
		SignKey:   "short", // 长度 <= 8
	}

	masked := MaskChainConfigSensitiveFields(config)

	if masked.SignKey != "sh***" {
		t.Errorf("SignKey should be masked even for short keys (first2+***), got: %s", masked.SignKey)
	}
}

func TestMaskChainConfigSensitiveFields_EmptyFields(t *testing.T) {
	config := &store.TenantChainConfig{
		ChainType: chainmakerStr,
		SignKey:   "",
	}

	masked := MaskChainConfigSensitiveFields(config)

	if masked.SignKey != "" {
		t.Errorf("empty SignKey should remain empty, got: %s", masked.SignKey)
	}
	// 原始对象也不应被修改
	if config.SignKey != "" {
		t.Errorf("original should not be modified")
	}
}

func TestMaskChainConfigSensitiveFields_Nil(t *testing.T) {
	// 不应 panic，应返回 nil
	result := MaskChainConfigSensitiveFields(nil)
	if result != nil {
		t.Errorf("expected nil for nil input, got: %v", result)
	}
}
