package gateway

import (
	"testing"
)

// ========== 合约配置字段校验测试 ==========

func TestValidateEthereumAddress(t *testing.T) {
	tests := []struct {
		name    string
		addr    string
		wantErr bool
	}{
		{"valid address", "0x742d35Cc6634C0532925a3b844Bc9e7595f2bD28", false},
		{"valid address lowercase", "0x742d35cc6634c0532925a3b844bc9e7595f2bd28", false},
		{"missing 0x prefix", "742d35Cc6634C0532925a3b844Bc9e7595f2bD28", true},
		{"too short", "0x742d35Cc6634C0532925a3b844Bc9e7595f2bD2", true},
		{"too long", "0x742d35Cc6634C0532925a3b844Bc9e7595f2bD288", true},
		{"invalid chars", "0x742d35Cc6634C0532925a3b844Bc9e7595f2bDGG", true},
		{"empty", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateEthereumAddress(tt.addr)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateEthereumAddress(%q) error = %v, wantErr %v", tt.addr, err, tt.wantErr)
			}
		})
	}
}

func TestValidateSolanaAddress(t *testing.T) {
	tests := []struct {
		name    string
		addr    string
		wantErr bool
	}{
		{"valid address", "11111111111111111111111111111111", false},
		{"valid program id", "TokenkegQfeZyiNwAJbNbGKPFXCWuBvf9Ss623VQ5DA", false},
		{"too short", "1111111111111111111111111111111", true},
		{"contains invalid char 0", "0111111111111111111111111111111111", true},
		{"contains invalid char O", "O111111111111111111111111111111111", true},
		{"contains invalid char I", "I111111111111111111111111111111111", true},
		{"contains invalid char l", "l111111111111111111111111111111111", true},
		{"empty", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateSolanaAddress(tt.addr)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateSolanaAddress(%q) error = %v, wantErr %v", tt.addr, err, tt.wantErr)
			}
		})
	}
}

func TestValidateABIJSON(t *testing.T) {
	tests := []struct {
		name    string
		abi     string
		wantErr bool
	}{
		{"valid array", `[{"type":"function","name":"transfer","inputs":[]}]`, false},
		{"valid object", `{"abi":[{"type":"function"}]}`, false},
		{"empty array", `[]`, false},
		{"invalid json", `not json`, true},
		{"incomplete json", `[{"type":`, true},
		{"empty string", ``, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateABIJSON(tt.abi)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateABIJSON(%q) error = %v, wantErr %v", tt.abi, err, tt.wantErr)
			}
		})
	}
}

func TestValidateChainType(t *testing.T) {
	tests := []struct {
		name      string
		chainType string
		wantErr   bool
	}{
		{"ethereum", "ethereum", false},
		{"Ethereum uppercase", "Ethereum", false},
		{"chainmaker", "chainmaker", false},
		{"solana", "solana", false},
		{"invalid type", "bitcoin", true},
		{"empty", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateChainType(tt.chainType)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateChainType(%q) error = %v, wantErr %v", tt.chainType, err, tt.wantErr)
			}
		})
	}
}

func TestValidateContractConfig_Ethereum(t *testing.T) {
	tests := []struct {
		name    string
		req     *ContractConfigRequest
		wantErr bool
	}{
		{
			"valid ethereum config",
			&ContractConfigRequest{
				ContractName: "MyToken",
				ContractAddr: "0x742d35Cc6634C0532925a3b844Bc9e7595f2bD28",
				AbiJSON:      `[{"type":"function","name":"transfer"}]`,
			},
			false,
		},
		{
			"missing contract addr",
			&ContractConfigRequest{
				ContractName: "MyToken",
				AbiJSON:      `[{"type":"function","name":"transfer"}]`,
			},
			true,
		},
		{
			"missing abi",
			&ContractConfigRequest{
				ContractName: "MyToken",
				ContractAddr: "0x742d35Cc6634C0532925a3b844Bc9e7595f2bD28",
			},
			true,
		},
		{
			"invalid address format",
			&ContractConfigRequest{
				ContractName: "MyToken",
				ContractAddr: "invalid_address",
				AbiJSON:      `[{"type":"function","name":"transfer"}]`,
			},
			true,
		},
		{
			"invalid abi format",
			&ContractConfigRequest{
				ContractName: "MyToken",
				ContractAddr: "0x742d35Cc6634C0532925a3b844Bc9e7595f2bD28",
				AbiJSON:      `not valid json`,
			},
			true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateContractConfig("ethereum", tt.req)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateContractConfig(ethereum, %+v) error = %v, wantErr %v", tt.req, err, tt.wantErr)
			}
		})
	}
}

func TestValidateContractConfig_Chainmaker(t *testing.T) {
	tests := []struct {
		name    string
		req     *ContractConfigRequest
		wantErr bool
	}{
		{
			"valid chainmaker config",
			&ContractConfigRequest{
				ContractName: "MyContract",
			},
			false,
		},
		{
			"with extra fields",
			&ContractConfigRequest{
				ContractName: "MyContract",
				ExtraConf:    `{"enable_subscribe":true,"deploy_block_height":100}`,
			},
			false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateContractConfig("chainmaker", tt.req)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateContractConfig(chainmaker, %+v) error = %v, wantErr %v", tt.req, err, tt.wantErr)
			}
		})
	}
}

func TestValidateContractConfig_Solana(t *testing.T) {
	tests := []struct {
		name    string
		req     *ContractConfigRequest
		wantErr bool
	}{
		{
			"valid solana config",
			&ContractConfigRequest{
				ContractName: "MyProgram",
				ContractAddr: "TokenkegQfeZyiNwAJbNbGKPFXCWuBvf9Ss623VQ5DA",
			},
			false,
		},
		{
			"missing program id",
			&ContractConfigRequest{
				ContractName: "MyProgram",
			},
			true,
		},
		{
			"invalid program id",
			&ContractConfigRequest{
				ContractName: "MyProgram",
				ContractAddr: "invalid",
			},
			true,
		},
		{
			"with valid extra conf",
			&ContractConfigRequest{
				ContractName: "MyProgram",
				ContractAddr: "TokenkegQfeZyiNwAJbNbGKPFXCWuBvf9Ss623VQ5DA",
				ExtraConf:    `{"solana_methods":[{"name":"transfer","accounts":["from","to"]}]}`,
			},
			false,
		},
		{
			"with invalid extra conf json",
			&ContractConfigRequest{
				ContractName: "MyProgram",
				ContractAddr: "TokenkegQfeZyiNwAJbNbGKPFXCWuBvf9Ss623VQ5DA",
				ExtraConf:    `not valid json`,
			},
			true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateContractConfig("solana", tt.req)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateContractConfig(solana, %+v) error = %v, wantErr %v", tt.req, err, tt.wantErr)
			}
		})
	}
}

func TestIsSupportedChainType(t *testing.T) {
	tests := []struct {
		chainType string
		want      bool
	}{
		{"ethereum", true},
		{"chainmaker", true},
		{"solana", true},
		{"Ethereum", true},
		{"SOLANA", true},
		{"bitcoin", false},
		{"", false},
		{"fabric", false},
	}

	for _, tt := range tests {
		t.Run(tt.chainType, func(t *testing.T) {
			got := IsSupportedChainType(tt.chainType)
			if got != tt.want {
				t.Errorf("IsSupportedChainType(%q) = %v, want %v", tt.chainType, got, tt.want)
			}
		})
	}
}
