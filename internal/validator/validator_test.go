package validator

import (
	"testing"

	"github.com/jackz-jones/blockchain-interactive-service/internal/store"
)

// ========== ValidateChainConfig 测试 ==========

func TestValidateChainConfig_ChainMaker_ValidPublicMode(t *testing.T) {
	req := &CreateChainConfigRequestBody{
		ChainType: "chainmaker",
		ChainId:   "chain1",
		AuthType:  "public",
		HashType:  "SHA256",
		SignKey:   "base64signkey",
		Nodes: []NodeRequest{
			{NodeAddr: "127.0.0.1:12301"},
		},
	}

	if err := ValidateChainConfig(req); err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
}

func TestValidateChainConfig_ChainMaker_ValidCertMode(t *testing.T) {
	req := &CreateChainConfigRequestBody{
		ChainType: "chainmaker",
		ChainId:   "chain1",
		AuthType:  "permissionedwithcert",
		OrgId:     "org1",
		HashType:  "SM3",
		SignKey:   "base64signkey",
		SignCert:  "base64cert",
		Nodes: []NodeRequest{
			{NodeAddr: "127.0.0.1:12301"},
		},
	}

	if err := ValidateChainConfig(req); err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
}

func TestValidateChainConfig_ChainMaker_MissingChainId(t *testing.T) {
	req := &CreateChainConfigRequestBody{
		ChainType: "chainmaker",
		AuthType:  "public",
		HashType:  "SHA256",
		SignKey:   "base64signkey",
		Nodes: []NodeRequest{
			{NodeAddr: "127.0.0.1:12301"},
		},
	}

	err := ValidateChainConfig(req)
	if err == nil {
		t.Fatal("expected error for missing chain_id")
	}
}

func TestValidateChainConfig_ChainMaker_MissingAuthType(t *testing.T) {
	req := &CreateChainConfigRequestBody{
		ChainType: "chainmaker",
		ChainId:   "chain1",
		HashType:  "SHA256",
		SignKey:   "base64signkey",
		Nodes: []NodeRequest{
			{NodeAddr: "127.0.0.1:12301"},
		},
	}

	err := ValidateChainConfig(req)
	if err == nil {
		t.Fatal("expected error for missing auth_type")
	}
}

func TestValidateChainConfig_ChainMaker_InvalidAuthType(t *testing.T) {
	req := &CreateChainConfigRequestBody{
		ChainType: "chainmaker",
		ChainId:   "chain1",
		AuthType:  "invalid_mode",
		HashType:  "SHA256",
		SignKey:   "base64signkey",
		Nodes: []NodeRequest{
			{NodeAddr: "127.0.0.1:12301"},
		},
	}

	err := ValidateChainConfig(req)
	if err == nil {
		t.Fatal("expected error for invalid auth_type")
	}
}

func TestValidateChainConfig_ChainMaker_MissingHashType(t *testing.T) {
	req := &CreateChainConfigRequestBody{
		ChainType: "chainmaker",
		ChainId:   "chain1",
		AuthType:  "public",
		SignKey:   "base64signkey",
		Nodes: []NodeRequest{
			{NodeAddr: "127.0.0.1:12301"},
		},
	}

	err := ValidateChainConfig(req)
	if err == nil {
		t.Fatal("expected error for missing hash_type")
	}
}

func TestValidateChainConfig_ChainMaker_InvalidHashType(t *testing.T) {
	req := &CreateChainConfigRequestBody{
		ChainType: "chainmaker",
		ChainId:   "chain1",
		AuthType:  "public",
		HashType:  "MD5",
		SignKey:   "base64signkey",
		Nodes: []NodeRequest{
			{NodeAddr: "127.0.0.1:12301"},
		},
	}

	err := ValidateChainConfig(req)
	if err == nil {
		t.Fatal("expected error for invalid hash_type")
	}
}

func TestValidateChainConfig_ChainMaker_EmptyNodes(t *testing.T) {
	req := &CreateChainConfigRequestBody{
		ChainType: "chainmaker",
		ChainId:   "chain1",
		AuthType:  "public",
		HashType:  "SHA256",
		SignKey:   "base64signkey",
		Nodes:     []NodeRequest{},
	}

	err := ValidateChainConfig(req)
	if err == nil {
		t.Fatal("expected error for empty nodes")
	}
}

func TestValidateChainConfig_ChainMaker_NodeMissingAddr(t *testing.T) {
	req := &CreateChainConfigRequestBody{
		ChainType: "chainmaker",
		ChainId:   "chain1",
		AuthType:  "public",
		HashType:  "SHA256",
		SignKey:   "base64signkey",
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
		ChainType: "chainmaker",
		ChainId:   "chain1",
		AuthType:  "permissionedwithcert",
		HashType:  "SHA256",
		SignKey:   "base64signkey",
		SignCert:  "base64cert",
		Nodes: []NodeRequest{
			{NodeAddr: "127.0.0.1:12301"},
		},
	}

	err := ValidateChainConfig(req)
	if err == nil {
		t.Fatal("expected error for missing org_id in cert mode")
	}
}

func TestValidateChainConfig_ChainMaker_CertModeMissingSignCert(t *testing.T) {
	req := &CreateChainConfigRequestBody{
		ChainType: "chainmaker",
		ChainId:   "chain1",
		AuthType:  "permissionedwithcert",
		OrgId:     "org1",
		HashType:  "SHA256",
		SignKey:   "base64signkey",
		Nodes: []NodeRequest{
			{NodeAddr: "127.0.0.1:12301"},
		},
	}

	err := ValidateChainConfig(req)
	if err == nil {
		t.Fatal("expected error for missing sign_cert in cert mode")
	}
}

func TestValidateChainConfig_ChainMaker_MissingSignKey(t *testing.T) {
	req := &CreateChainConfigRequestBody{
		ChainType: "chainmaker",
		ChainId:   "chain1",
		AuthType:  "public",
		HashType:  "SHA256",
		Nodes: []NodeRequest{
			{NodeAddr: "127.0.0.1:12301"},
		},
	}

	err := ValidateChainConfig(req)
	if err == nil {
		t.Fatal("expected error for missing sign_key")
	}
}

func TestValidateChainConfig_Ethereum_Valid(t *testing.T) {
	req := &CreateChainConfigRequestBody{
		ChainType: "ethereum",
		HttpUrl:   "http://localhost:8545",
	}

	if err := ValidateChainConfig(req); err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
}

func TestValidateChainConfig_Ethereum_MissingHttpUrl(t *testing.T) {
	req := &CreateChainConfigRequestBody{
		ChainType: "ethereum",
	}

	err := ValidateChainConfig(req)
	if err == nil {
		t.Fatal("expected error for missing http_url")
	}
}

func TestValidateChainConfig_Solana_Valid(t *testing.T) {
	req := &CreateChainConfigRequestBody{
		ChainType: "solana",
		SolRpcUrl: "http://localhost:8899",
	}

	if err := ValidateChainConfig(req); err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
}

func TestValidateChainConfig_Solana_MissingRpcUrl(t *testing.T) {
	req := &CreateChainConfigRequestBody{
		ChainType: "solana",
	}

	err := ValidateChainConfig(req)
	if err == nil {
		t.Fatal("expected error for missing sol_rpc_url")
	}
}

// ========== MaskChainConfigSensitiveFields 测试 ==========

func TestMaskChainConfigSensitiveFields_ChainMaker(t *testing.T) {
	config := &store.TenantChainConfig{
		ChainType:  "chainmaker",
		SignKey:    "abcdefghijklmnop1234567890",
		UserTlsKey: "tlskey1234567890abcdef",
		UserEncKey: "enckey1234567890abcdef",
		ChainId:    "chain1",
	}

	masked := MaskChainConfigSensitiveFields(config)

	// 原始对象不应被修改
	if config.SignKey != "abcdefghijklmnop1234567890" {
		t.Errorf("original SignKey should not be modified, got: %s", config.SignKey)
	}

	// 私钥不出域，副本中私钥字段应为空
	if masked.SignKey != "" {
		t.Errorf("SignKey should be empty in response, got: %s", masked.SignKey)
	}
	if masked.UserTlsKey != "" {
		t.Errorf("UserTlsKey should be empty in response, got: %s", masked.UserTlsKey)
	}
	if masked.UserEncKey != "" {
		t.Errorf("UserEncKey should be empty in response, got: %s", masked.UserEncKey)
	}
	// 非敏感字段保持不变
	if masked.ChainId != "chain1" {
		t.Errorf("ChainId should not be affected, got: %s", masked.ChainId)
	}
}

func TestMaskChainConfigSensitiveFields_Ethereum(t *testing.T) {
	config := &store.TenantChainConfig{
		ChainType:  "ethereum",
		PrivateKey: "0x1234567890abcdef1234567890abcdef",
		HttpUrl:    "http://localhost:8545",
	}

	masked := MaskChainConfigSensitiveFields(config)

	if masked.PrivateKey != "" {
		t.Errorf("PrivateKey should be empty in response, got: %s", masked.PrivateKey)
	}
	if masked.HttpUrl != "http://localhost:8545" {
		t.Errorf("HttpUrl should not be affected, got: %s", masked.HttpUrl)
	}
}

func TestMaskChainConfigSensitiveFields_Solana(t *testing.T) {
	config := &store.TenantChainConfig{
		ChainType:     "solana",
		SolPrivateKey: "5KQwrPbwdL6PhXujxW37FSSQZ1JiwsST4cqQzDeyXtP79zkvFD3",
		SolRpcUrl:     "http://localhost:8899",
	}

	masked := MaskChainConfigSensitiveFields(config)

	if masked.SolPrivateKey != "" {
		t.Errorf("SolPrivateKey should be empty in response, got: %s", masked.SolPrivateKey)
	}
	if masked.SolRpcUrl != "http://localhost:8899" {
		t.Errorf("SolRpcUrl should not be affected, got: %s", masked.SolRpcUrl)
	}
}

func TestMaskChainConfigSensitiveFields_ShortKey(t *testing.T) {
	config := &store.TenantChainConfig{
		ChainType: "chainmaker",
		SignKey:   "short", // 长度 <= 8
	}

	masked := MaskChainConfigSensitiveFields(config)

	if masked.SignKey != "" {
		t.Errorf("SignKey should be empty in response regardless of length, got: %s", masked.SignKey)
	}
}

func TestMaskChainConfigSensitiveFields_EmptyFields(t *testing.T) {
	config := &store.TenantChainConfig{
		ChainType: "chainmaker",
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
