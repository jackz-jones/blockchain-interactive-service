package validator

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	chainmakersdk "chainmaker.org/chainmaker/sdk-go/v2"

	"github.com/jackz-jones/blockchain-interactive-service/internal/store"
	pb "github.com/jackz-jones/blockchain-interactive-service/pb"
)

// 链类型常量，直接引用 pb 枚举定义
var (
	ChainTypeChainmaker = strings.ToLower(pb.ChainType_Chainmaker.String())
	ChainTypeEthereum   = strings.ToLower(pb.ChainType_Ethereum.String())
	ChainTypeSolana     = strings.ToLower(pb.ChainType_Solana.String())
)

// NodeRequest 节点配置请求体
type NodeRequest struct {
	NodeAddr    string `json:"node_addr"`
	ConnCnt     int    `json:"conn_cnt"`
	EnableTls   bool   `json:"enable_tls"`
	TlsHostName string `json:"tls_host_name"`
	CaCert      string `json:"ca_cert"`
}

// CreateChainConfigRequestBody 创建链配置请求体
type CreateChainConfigRequestBody struct {
	ChainName string `json:"chain_name"`
	ChainType string `json:"chain_type"`
	Enable    bool   `json:"enable"`

	// ChainMaker 专属字段
	ChainId     string `json:"chain_id"`
	AuthType    string `json:"auth_type"`
	OrgId       string `json:"org_id"`
	HashType    string `json:"hash_type"`
	SignKey     string `json:"sign_key"`
	SignCert    string `json:"sign_cert"`
	UserTlsKey  string `json:"user_tls_key"`
	UserTlsCert string `json:"user_tls_cert"`
	UserEncKey  string `json:"user_enc_key"`
	UserEncCert string `json:"user_enc_cert"`
	ProxyUrl    string `json:"proxy_url"`

	// Ethereum 专属字段
	EthChainId   int64  `json:"eth_chain_id"`
	HttpUrl      string `json:"http_url"`
	WebsocketUrl string `json:"websocket_url"`
	PrivateKey   string `json:"private_key"`

	// Solana 专属字段
	SolRpcUrl       string `json:"sol_rpc_url"`
	SolPrivateKey   string `json:"sol_private_key"`
	CommitmentLevel string `json:"commitment_level"`
	SkipPreflight   bool   `json:"skip_preflight"`
	MaxRetries      int    `json:"max_retries"`

	// 节点配置（ChainMaker 专用）
	Nodes []NodeRequest `json:"nodes"`
}

// ContractConfigRequest 合约配置请求体
type ContractConfigRequest struct {
	ContractName    string `json:"contract_name"`    // 合约名称（必填）
	ContractAddr    string `json:"contract_addr"`    // 合约地址
	AbiJSON         string `json:"abi_json"`         // ABI JSON
	EnableSubscribe bool   `json:"enable_subscribe"` // 是否开启事件订阅
	ExtraConf       string `json:"extra_conf"`       // 额外配置 JSON（包含 deployBlockHeight 等）
}

// IsSupportedChainType 检查链类型是否在支持列表中（大小写不敏感）
func IsSupportedChainType(chainType string) bool {
	for name := range pb.ChainType_value {
		if strings.EqualFold(name, chainType) {
			return true
		}
	}
	return false
}

// ValidateContractConfig 根据链类型校验合约配置字段
func ValidateContractConfig(chainType string, req *ContractConfigRequest) error {
	ct := strings.ToLower(chainType)

	switch ct {
	case ChainTypeEthereum:
		return validateEthereumContract(req)
	case ChainTypeChainmaker:
		return validateChainmakerContract(req)
	case ChainTypeSolana:
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
	return validateABIJSON(req.AbiJSON)
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
	// 如果提供了 ExtraConf，校验其为合法 JSON
	if req.ExtraConf != "" {
		var extra map[string]interface{}
		if err := json.Unmarshal([]byte(req.ExtraConf), &extra); err != nil {
			return fmt.Errorf("extra_conf must be valid JSON: %v", err)
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

// ValidateChainConfig 根据链类型校验链配置请求体中的结构化字段
func ValidateChainConfig(req *CreateChainConfigRequestBody) error {
	ct := strings.ToLower(req.ChainType)

	switch ct {
	case ChainTypeChainmaker:
		return validateChainMakerConfig(req)
	case ChainTypeEthereum:
		return validateEthereumConfig(req)
	case ChainTypeSolana:
		return validateSolanaConfig(req)
	default:
		return nil
	}
}

// validateChainMakerConfig 校验 ChainMaker 链配置必填字段
func validateChainMakerConfig(req *CreateChainConfigRequestBody) error {
	if req.ChainId == "" {
		return fmt.Errorf("chain_id is required for chainmaker")
	}

	if req.AuthType == "" {
		return fmt.Errorf("auth_type is required for chainmaker")
	}

	// 校验 AuthType 枚举值，使用 chainmaker SDK 定义的合法值
	if _, ok := chainmakersdk.StringToAuthTypeMap[req.AuthType]; !ok {
		return fmt.Errorf("auth_type is invalid, got '%s'", req.AuthType)
	}

	if req.HashType == "" {
		return fmt.Errorf("hash_type is required for chainmaker")
	}

	// 校验 HashType 枚举值
	validHashTypes := map[string]bool{"SHA256": true, "SM3": true, "SHA3_256": true}
	if !validHashTypes[req.HashType] {
		return fmt.Errorf("hash_type must be 'SHA256', 'SM3' or 'SHA3_256', got '%s'", req.HashType)
	}

	// 至少一个节点配置
	if len(req.Nodes) == 0 {
		return fmt.Errorf("nodes must contain at least one node for chainmaker")
	}

	// 校验每个节点配置
	for i, node := range req.Nodes {
		if node.NodeAddr == "" {
			return fmt.Errorf("nodes[%d].node_addr is required", i)
		}
	}

	// 签名私钥必填（当前项目要求用户必须配置链账户私钥）
	if req.SignKey == "" {
		return fmt.Errorf("sign_key is required for chainmaker (signing key is needed for chain operations)")
	}

	// 证书模式下额外校验
	if chainmakersdk.StringToAuthTypeMap[req.AuthType] == chainmakersdk.PermissionedWithCert {
		if req.OrgId == "" {
			return fmt.Errorf("org_id is required when auth_type is 'permissionedwithcert'")
		}
		if req.SignCert == "" {
			return fmt.Errorf("sign_cert is required when auth_type is 'permissionedwithcert'")
		}
	}

	return nil
}

// validateEthereumConfig 校验 Ethereum 链配置必填字段
func validateEthereumConfig(req *CreateChainConfigRequestBody) error {
	if req.HttpUrl == "" {
		return fmt.Errorf("http_url is required for ethereum")
	}
	return nil
}

// validateSolanaConfig 校验 Solana 链配置必填字段
func validateSolanaConfig(req *CreateChainConfigRequestBody) error {
	if req.SolRpcUrl == "" {
		return fmt.Errorf("sol_rpc_url is required for solana")
	}
	return nil
}

// ValidateChainType 校验链类型是否合法
func ValidateChainType(chainType string) error {
	if !IsSupportedChainType(chainType) {
		// 构建支持的类型列表字符串
		types := make([]string, 0, len(pb.ChainType_value))
		for t := range pb.ChainType_value {
			types = append(types, strings.ToLower(t))
		}
		return fmt.Errorf("unsupported chain_type '%s', supported types: %s",
			chainType, strings.Join(types, ", "))
	}
	return nil
}

// MaskChainConfigSensitiveFields 返回链配置的脱敏副本，不修改原始对象
// 私钥不能出域，API 响应中对私钥字段进行脱敏处理，保留前后部分字符，中间用 * 替代
func MaskChainConfigSensitiveFields(config *store.TenantChainConfig) *store.TenantChainConfig {
	if config == nil {
		return nil
	}

	// 浅拷贝一份副本
	masked := *config

	// 私钥不出域，进行脱敏处理（保留前后部分字符，中间用 * 替代）
	masked.SignKey = MaskSensitiveString(config.SignKey)
	masked.UserTlsKey = MaskSensitiveString(config.UserTlsKey)
	masked.UserEncKey = MaskSensitiveString(config.UserEncKey)
	masked.PrivateKey = MaskSensitiveString(config.PrivateKey)
	masked.SolPrivateKey = MaskSensitiveString(config.SolPrivateKey)

	return &masked
}

// MaskSensitiveString 对敏感字符串进行脱敏处理
// 如果字符串长度 <= 8，则保留前2个字符，其余用 * 替代
// 如果字符串长度 > 8，则保留前4个和后4个字符，中间用 **** 替代
// 如果字符串为空，则返回空字符串
func MaskSensitiveString(s string) string {
	if s == "" {
		return ""
	}
	length := len(s)
	if length <= 8 {
		if length <= 2 {
			return s[:1] + strings.Repeat("*", length-1)
		}
		return s[:2] + strings.Repeat("*", length-2)
	}
	return s[:4] + "****" + s[length-4:]
}
