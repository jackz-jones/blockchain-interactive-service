package config

import (
	"errors"
	"fmt"
	"strings"

	"github.com/ethereum/go-ethereum/crypto"
	"github.com/gagliardetto/solana-go"
	"github.com/zeromicro/go-zero/zrpc"
)

type Config struct {
	zrpc.RpcServerConf
	GrpcConf      GrpcConf
	SubscribeConf SubscribeConf

	// 数据库配置（多租户持久化存储）
	DatabaseConf DatabaseConf

	// 本地需要交互的链配置列表
	ChainConfs map[string]*ChainConf
}

// DatabaseConf 数据库配置
type DatabaseConf struct {
	// Driver 数据库驱动：postgres、mysql
	Driver string `json:",default=postgres"`

	// Host 数据库主机地址
	Host string `json:",default=localhost"`

	// Port 数据库端口
	Port int `json:",default=5432"`

	// User 数据库用户名
	User string `json:",default=postgres"`

	// Password 数据库密码
	Password string `json:",optional"`

	// DBName 数据库名
	DBName string `json:",default=chain_interactive"`

	// SSLMode SSL 模式（postgres 专用）
	SSLMode string `json:",default=disable"`

	// AutoMigrate 是否自动迁移表结构
	AutoMigrate bool `json:",default=true"`
}

// GrpcConf contain all config items for grpc server initiation
type GrpcConf struct {
	// CaCertFile 是 CA 根证书文件的路径
	CaCertFile string

	// ServerCertFile 是服务端证书文件的路径
	ServerCertFile string

	// ServerKeyFile 是服务端私钥文件的路径
	ServerKeyFile string

	// MaxRecvMsgSize 是最大接收消息大小
	MaxRecvMsgSize int

	// MaxSendMsgSize 是最大发送消息大小
	MaxSendMsgSize int
}

// SubscribeConf contain all config items for subscribing chain event
type SubscribeConf struct {

	// confType 配置类型（cluster或者node）
	ConfType string

	// RedisAddr 是 Redis 服务器地址
	RedisAddr string

	// RedisUserName 是 Redis 用户名
	RedisUserName string

	// RedisPassword 是 Redis 密码
	RedisPassword string

	// nolint:staticcheck
	// 哨兵模式的MasterName，其他模式可忽略
	MasterName string `json:",optional"`
}

// ChainConf contain all config items for chain config
type ChainConf struct {

	// Enable 是否启用该链
	Enable bool

	// ChainType 链类型,枚举值：ethereum、chainmaker、solana
	ChainType string

	// 连接链需要的配置信息
	SdkConf SdkConf

	// ContractConfs 当前链下的合约配置列表
	ContractConfs map[string]*ContractConf
}

// SdkConf contain all config items for chain sdk
type SdkConf struct {

	// nolint:staticcheck
	// ConfFilePath sdk 具体配置文件路径，长安链需要指定
	ConfFilePath string `json:",optional"`

	// nolint:staticcheck
	// EthConf 以太坊链需要的配置信息
	EthConf EthConf `json:",optional"`

	// nolint:staticcheck
	// SolanaConf Solana链需要的配置信息
	SolanaConf SolanaConf `json:",optional"`
}

// EthConf contain all config items for ethereum chain
type EthConf struct {

	// 以太坊链 id
	ChainId int64

	// 节点 http url，用于发送交易
	HttpUrl string

	// 节点 websocket url，用于订阅事件
	WebsocketUrl string

	// 私钥 hex 字符串，用于交易签名
	PrivateKey string

	// gas limit
	GasLimit uint64
}

// SolanaConf contain all config items for solana chain
type SolanaConf struct {

	// RpcUrl Solana 节点 RPC URL
	RpcUrl string

	// PrivateKey 私钥 base58 字符串，用于交易签名
	PrivateKey string

	// CommitmentLevel 确认级别：processed、confirmed、finalized
	CommitmentLevel string

	// SkipPreflight 是否跳过预检
	SkipPreflight bool

	// MaxRetries 最大重试次数
	MaxRetries int
}

// SolanaAccountMeta Solana 指令中需要的账户元数据
type SolanaAccountMeta struct {
	// Pubkey 账户公钥 base58 字符串；支持特殊占位符 "$fromAddress" 表示发送方地址
	Pubkey string

	// IsSigner 该账户是否需要签名
	IsSigner bool

	// IsWritable 该账户是否可写
	IsWritable bool
}

// SolanaArgSpec Borsh 序列化的参数类型描述
type SolanaArgSpec struct {
	// Name 参数名称（与调用时 KeyValuePair.Key 对应）
	Name string

	// Type 参数类型，枚举：u8、u16、u32、u64、i64、bool、string、pubkey、bytes
	Type string
}

// SolanaMethodSpec Solana 合约某个方法的调用规范
type SolanaMethodSpec struct {
	// Discriminator 8 字节方法判别符的 hex 字符串（如 Anchor 生成的 discriminator），必填
	Discriminator string

	// ArgSchema 参数顺序及类型，构建 Borsh 序列化 instruction data 时按此顺序
	// nolint:staticcheck
	ArgSchema []SolanaArgSpec `json:",optional"`

	// Accounts Invoke 调用时的账户列表
	// nolint:staticcheck
	Accounts []SolanaAccountMeta `json:",optional"`

	// QueryAccounts Query 调用时要读取的账户公钥列表（base58，支持 "$fromAddress" 占位符）
	// nolint:staticcheck
	QueryAccounts []string `json:",optional"`
}

// ContractConf contain all config items for contract
type ContractConf struct {

	// EnableSubscribe 是否开启订阅
	EnableSubscribe bool

	// ContractType 合约类型,枚举值：notification、nft
	ContractType string

	// nolint:staticcheck
	// ContractName 合约名称，长安链上调用需要合约名称
	ContractName string `json:",optional"`

	// DeployBlockHeight 合约部署高度，长安链和以太坊上订阅都需要合约部署高度
	DeployBlockHeight uint64

	// nolint:staticcheck
	// ContractAddr 合约地址，以太坊上需要
	ContractAddr string `json:",optional"`

	// nolint:staticcheck
	// Abi 合约的abi json 文件，以太坊上需要
	Abi string `json:",optional"`

	// nolint:staticcheck
	// GetHistoryEventInterval 轮训以太坊历史事件的间隔时间：ms
	GetHistoryEventInterval uint64 `json:",optional"`

	// nolint:staticcheck
	// GetHistoryEventHeightWindow 轮训以太坊历史事件的区块高度窗口大小
	GetHistoryEventHeightWindow uint64 `json:",optional"`

	// nolint:staticcheck
	// SolanaMethods Solana 方法调用规范：methodName -> MethodSpec
	SolanaMethods map[string]SolanaMethodSpec `json:",optional"`
}

// 链类型常量
const (
	ChainTypeEthereum   = "ethereum"
	ChainTypeChainMaker = "chainmaker"
	ChainTypeSolana     = "solana"
)

// supportedChainTypes 支持的链类型枚举集合
var supportedChainTypes = map[string]struct{}{
	ChainTypeEthereum:   {},
	ChainTypeChainMaker: {},
	ChainTypeSolana:     {},
}

// Validate validate config
func (c *Config) Validate() error {
	if c.ListenOn == "" {
		return errors.New("listen address is required")
	}

	if len(c.ChainConfs) == 0 {
		return errors.New("at least one chain configuration is required")
	}

	for chainName, chainConf := range c.ChainConfs {
		if err := c.validateChainConf(chainName, chainConf); err != nil {
			return err
		}
	}

	return nil
}

// validateChainConf 校验单条链的配置
func (c *Config) validateChainConf(chainName string, chainConf *ChainConf) error {
	if chainConf == nil {
		return fmt.Errorf("chain[%s] config is nil", chainName)
	}

	if !chainConf.Enable {
		return nil
	}

	// 1) 校验链类型
	ct := strings.ToLower(chainConf.ChainType)
	if _, ok := supportedChainTypes[ct]; !ok {
		return fmt.Errorf("chain[%s] has unsupported chainType: %s", chainName, chainConf.ChainType)
	}

	// 2) 校验链级 SDK 配置
	if err := validateSDKConf(chainName, ct, &chainConf.SdkConf); err != nil {
		return err
	}

	// 3) 校验合约配置
	for contractName, contractConf := range chainConf.ContractConfs {
		if err := validateContractConf(chainName, contractName, ct, contractConf); err != nil {
			return err
		}
	}

	return nil
}

// validateSDKConf 校验链级 SDK 配置
func validateSDKConf(chainName, chainType string, sdkConf *SdkConf) error {
	switch chainType {
	case ChainTypeEthereum:
		if sdkConf.EthConf.HttpUrl == "" {
			return fmt.Errorf("chain[%s] ethereum HttpUrl is required", chainName)
		}
		if sdkConf.EthConf.PrivateKey != "" {
			if _, err := crypto.HexToECDSA(sdkConf.EthConf.PrivateKey); err != nil {
				return fmt.Errorf("chain[%s] ethereum PrivateKey invalid: %v", chainName, err)
			}
		}
	case ChainTypeChainMaker:
		if sdkConf.ConfFilePath == "" {
			return fmt.Errorf("chain[%s] chainmaker ConfFilePath is required", chainName)
		}
	case ChainTypeSolana:
		if sdkConf.SolanaConf.RpcUrl == "" {
			return fmt.Errorf("chain[%s] solana RpcUrl is required", chainName)
		}
		if sdkConf.SolanaConf.PrivateKey != "" {
			if _, err := solana.PrivateKeyFromBase58(sdkConf.SolanaConf.PrivateKey); err != nil {
				return fmt.Errorf("chain[%s] solana PrivateKey invalid: %v", chainName, err)
			}
		}
	}
	return nil
}

// validateContractConf 校验合约配置
func validateContractConf(chainName, contractName, chainType string, contractConf *ContractConf) error {
	if contractConf == nil {
		return fmt.Errorf("chain[%s] contract[%s] config is nil", chainName, contractName)
	}

	if !contractConf.EnableSubscribe {
		return nil
	}

	// 订阅场景下必填字段校验
	switch chainType {
	case ChainTypeEthereum:
		if contractConf.ContractAddr == "" {
			return fmt.Errorf("chain[%s] contract[%s] ContractAddr required for ethereum subscribe",
				chainName, contractName)
		}
		if contractConf.GetHistoryEventInterval == 0 {
			return fmt.Errorf("chain[%s] contract[%s] GetHistoryEventInterval must be > 0",
				chainName, contractName)
		}
		if contractConf.GetHistoryEventHeightWindow == 0 {
			return fmt.Errorf("chain[%s] contract[%s] GetHistoryEventHeightWindow must be > 0",
				chainName, contractName)
		}
	case ChainTypeChainMaker:
		if contractConf.ContractName == "" {
			return fmt.Errorf("chain[%s] contract[%s] ContractName required for chainmaker subscribe",
				chainName, contractName)
		}
	case ChainTypeSolana:
		if contractConf.ContractAddr == "" {
			return fmt.Errorf("chain[%s] contract[%s] ContractAddr required for solana subscribe",
				chainName, contractName)
		}
	}
	return nil
}
