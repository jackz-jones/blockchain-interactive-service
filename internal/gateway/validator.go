package gateway

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
)

// 支持的链类型列表
var SupportedChainTypes = []string{"ethereum", "chainmaker", "solana"}

// IsSupportedChainType 检查链类型是否在支持列表中
func IsSupportedChainType(chainType string) bool {
	ct := strings.ToLower(chainType)
	for _, t := range SupportedChainTypes {
		if ct == t {
			return true
		}
	}
	return false
}

// ValidateContractConfig 根据链类型校验合约配置字段
func ValidateContractConfig(chainType string, req *ContractConfigRequest) error {
	ct := strings.ToLower(chainType)

	switch ct {
	case "ethereum":
		return validateEthereumContract(req)
	case "chainmaker":
		return validateChainmakerContract(req)
	case "solana":
		return validateSolanaContract(req)
	default:
		// 未知链类型，只做基础校验
		return nil
	}
}

// validateEthereumContract 校验以太坊合约配置
// 必填：contractAddr、abiJson
func validateEthereumContract(req *ContractConfigRequest) error {
	if req.ContractAddr == "" {
		return fmt.Errorf("contract_addr is required for ethereum chain type")
	}
	if err := validateEthereumAddress(req.ContractAddr); err != nil {
		return err
	}
	if req.AbiJSON == "" {
		return fmt.Errorf("abi_json is required for ethereum chain type")
	}
	if err := validateABIJSON(req.AbiJSON); err != nil {
		return err
	}
	return nil
}

// validateChainmakerContract 校验长安链合约配置
// 必填：contractName（已在上层校验）
func validateChainmakerContract(req *ContractConfigRequest) error {
	// chainmaker 只需要 contractName，已在上层校验
	return nil
}

// validateSolanaContract 校验 Solana 合约配置
// 必填：contractAddr（程序 ID）
func validateSolanaContract(req *ContractConfigRequest) error {
	if req.ContractAddr == "" {
		return fmt.Errorf("contract_addr (program ID) is required for solana chain type")
	}
	if err := validateSolanaAddress(req.ContractAddr); err != nil {
		return err
	}
	// 如果提供了 ExtraConf，校验 solanaMethods 格式
	if req.ExtraConf != "" {
		var extra map[string]interface{}
		if err := json.Unmarshal([]byte(req.ExtraConf), &extra); err != nil {
			return fmt.Errorf("extra_conf must be valid JSON: %v", err)
		}
		if methods, ok := extra["solana_methods"]; ok {
			// 确保 solana_methods 是有效的 JSON 格式
			methodsJSON, err := json.Marshal(methods)
			if err != nil {
				return fmt.Errorf("solana_methods format invalid: %v", err)
			}
			_ = methodsJSON
		}
	}
	return nil
}

// ethereumAddrRegex 以太坊地址正则：0x 前缀 + 40 位十六进制字符
var ethereumAddrRegex = regexp.MustCompile(`^0x[0-9a-fA-F]{40}$`)

// validateEthereumAddress 校验以太坊地址格式
func validateEthereumAddress(addr string) error {
	if !ethereumAddrRegex.MatchString(addr) {
		return fmt.Errorf("invalid ethereum address format: must start with '0x' followed by 40 hex characters")
	}
	return nil
}

// solanaAddrRegex Solana 地址正则：Base58 编码，32-44 个字符
var solanaAddrRegex = regexp.MustCompile(`^[1-9A-HJ-NP-Za-km-z]{32,44}$`)

// validateSolanaAddress 校验 Solana 地址格式（Base58 编码）
func validateSolanaAddress(addr string) error {
	if !solanaAddrRegex.MatchString(addr) {
		return fmt.Errorf("invalid solana address format: must be a valid Base58 encoded string (32-44 characters)")
	}
	return nil
}

// validateABIJSON 校验 ABI JSON 格式
func validateABIJSON(abiStr string) error {
	// ABI 应该是一个 JSON 数组
	var abiArray []interface{}
	if err := json.Unmarshal([]byte(abiStr), &abiArray); err != nil {
		// 也尝试解析为对象格式（某些工具生成的 ABI 可能是对象）
		var abiObj map[string]interface{}
		if err2 := json.Unmarshal([]byte(abiStr), &abiObj); err2 != nil {
			return fmt.Errorf("abi_json format invalid: must be a valid JSON array or object")
		}
	}
	return nil
}

// ValidateChainType 校验链类型是否合法
func ValidateChainType(chainType string) error {
	if !IsSupportedChainType(chainType) {
		return fmt.Errorf("unsupported chain_type '%s', supported types: %s",
			chainType, strings.Join(SupportedChainTypes, ", "))
	}
	return nil
}
