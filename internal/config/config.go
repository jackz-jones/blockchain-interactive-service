package config

import (
	"errors"

	"github.com/zeromicro/go-zero/zrpc"
)

type Config struct {
	zrpc.RpcServerConf
	GrpcConf      GrpcConf
	SubscribeConf SubscribeConf

	// 本地需要交互的链配置列表
	ChainConfs map[string]*ChainConf
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

	// ChainType 链类型,枚举值：ethereum、chainmaker
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
}

// Validate validate config
func (c *Config) Validate() error {
	if c.ListenOn == "" {
		return errors.New("listen address is required")
	}

	if len(c.ChainConfs) == 0 {
		return errors.New("at least one chain configuration is required")
	}

	// todo: 添加更多验证逻辑
	return nil
}
