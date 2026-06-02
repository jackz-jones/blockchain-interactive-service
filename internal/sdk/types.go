package sdk

// ChainConf 链运行时配置（从 DB 加载后构建）
type ChainConf struct {
	// Enable 是否启用该链
	Enable bool

	// ChainType 链类型,枚举值：ethereum、chainmaker、solana
	ChainType string

	// 连接链需要的配置信息
	SDKConf SDKConf

	// ContractConfs 当前链下的合约配置列表
	ContractConfs map[string]*ContractConf
}

// SDKConf 链 SDK 连接配置
//
//nolint:revive // SDKConf 语义明确，改为 Conf 过于通用
type SDKConf struct {
	// EthConf 以太坊链需要的配置信息
	EthConf EthConf `json:"EthConf,omitempty"`

	// SolanaConf Solana链需要的配置信息
	SolanaConf SolanaConf `json:"SolanaConf,omitempty"`

	// ChainMakerConf 长安链结构化配置信息
	ChainMakerConf ChainMakerConf `json:"ChainMakerConf,omitempty"`
}

// ChainMakerConf 长安链 SDK 连接配置（结构化方式，替代 YAML 配置文件）
type ChainMakerConf struct {
	// ChainId 链 ID
	ChainId string `json:"ChainId,omitempty"`

	// AuthType 认证模式：public（公钥模式）/ permissionedwithcert（证书模式）
	AuthType string `json:"AuthType,omitempty"`

	// OrgId 组织 ID（证书模式下必填）
	OrgId string `json:"OrgId,omitempty"`

	// HashType 哈希算法：SHA256 / SM3 / SHA3_256
	HashType string `json:"HashType,omitempty"`

	// SignKey 签名私钥（base64 编码字符串）
	SignKey string `json:"SignKey,omitempty"`

	// SignCert 签名证书（base64 编码字符串，证书模式下必填）
	SignCert string `json:"SignCert,omitempty"`

	// UserTlsKey TLS 私钥（base64 编码字符串，开启 TLS 时需要）
	UserTlsKey string `json:"UserTlsKey,omitempty"`

	// UserTlsCert TLS 证书（base64 编码字符串，开启 TLS 时需要）
	UserTlsCert string `json:"UserTlsCert,omitempty"`

	// UserEncKey 国密加密私钥（base64 编码字符串，可选）
	UserEncKey string `json:"UserEncKey,omitempty"`

	// UserEncCert 国密加密证书（base64 编码字符串，可选）
	UserEncCert string `json:"UserEncCert,omitempty"`

	// Nodes 节点配置列表
	Nodes []ChainMakerNodeConf `json:"Nodes,omitempty"`
}

// ChainMakerNodeConf 长安链节点配置
type ChainMakerNodeConf struct {
	// NodeAddr 节点地址（格式 IP:Port）
	NodeAddr string `json:"NodeAddr,omitempty"`

	// ConnCnt 连接数（默认 10）
	ConnCnt int `json:"ConnCnt,omitempty"`

	// EnableTls 是否开启 TLS
	EnableTls bool `json:"EnableTls,omitempty"`

	// TlsHostName TLS 主机名
	TlsHostName string `json:"TlsHostName,omitempty"`

	// CaCert 节点 CA 证书（base64 编码字符串，开启 TLS 时必填）
	CaCert string `json:"CaCert,omitempty"`
}

// EthConf 以太坊链连接配置
type EthConf struct {
	// 以太坊链 id
	ChainId int64 `json:"ChainId,omitempty"`

	// 节点 http url，用于发送交易
	HttpUrl string `json:"HttpUrl,omitempty"`

	// 节点 websocket url，用于订阅事件
	WebsocketUrl string `json:"WebsocketUrl,omitempty"`

	// 私钥 hex 字符串，用于交易签名
	PrivateKey string `json:"PrivateKey,omitempty"`

	// gas limit
	GasLimit uint64 `json:"GasLimit,omitempty"`
}

// SolanaConf Solana 链连接配置
type SolanaConf struct {
	// RpcUrl Solana 节点 RPC URL
	RpcUrl string `json:"RpcUrl,omitempty"`

	// PrivateKey 私钥 base58 字符串，用于交易签名
	PrivateKey string `json:"PrivateKey,omitempty"`

	// CommitmentLevel 确认级别：processed、confirmed、finalized
	CommitmentLevel string `json:"CommitmentLevel,omitempty"`

	// SkipPreflight 是否跳过预检
	SkipPreflight bool `json:"SkipPreflight,omitempty"`

	// MaxRetries 最大重试次数
	MaxRetries int `json:"MaxRetries,omitempty"`
}

// SolanaAccountMeta Solana 指令中需要的账户元数据
type SolanaAccountMeta struct {
	// Pubkey 账户公钥 base58 字符串；支持特殊占位符 "$fromAddress" 表示发送方地址
	Pubkey string `json:"Pubkey,omitempty"`

	// IsSigner 该账户是否需要签名
	IsSigner bool `json:"IsSigner,omitempty"`

	// IsWritable 该账户是否可写
	IsWritable bool `json:"IsWritable,omitempty"`
}

// SolanaArgSpec Borsh 序列化的参数类型描述
type SolanaArgSpec struct {
	// Name 参数名称（与调用时 KeyValuePair.Key 对应）
	Name string `json:"Name,omitempty"`

	// Type 参数类型，枚举：u8、u16、u32、u64、i64、bool、string、pubkey、bytes
	Type string `json:"Type,omitempty"`
}

// SolanaMethodSpec Solana 合约某个方法的调用规范
type SolanaMethodSpec struct {
	// Discriminator 8 字节方法判别符的 hex 字符串（如 Anchor 生成的 discriminator），必填
	Discriminator string `json:"Discriminator,omitempty"`

	// ArgSchema 参数顺序及类型，构建 Borsh 序列化 instruction data 时按此顺序
	ArgSchema []SolanaArgSpec `json:"ArgSchema,omitempty"`

	// Accounts Invoke 调用时的账户列表
	Accounts []SolanaAccountMeta `json:"Accounts,omitempty"`

	// QueryAccounts Query 调用时要读取的账户公钥列表（base58，支持 "$fromAddress" 占位符）
	QueryAccounts []string `json:"QueryAccounts,omitempty"`
}

// ContractConf 合约运行时配置（从 DB 加载后构建）
type ContractConf struct {
	// EnableSubscribe 是否开启订阅
	EnableSubscribe bool `json:"EnableSubscribe,omitempty"`

	// ContractName 合约名称，长安链上调用需要合约名称
	ContractName string `json:"ContractName,omitempty"`

	// DeployBlockHeight 合约部署高度，长安链和以太坊上订阅都需要合约部署高度
	DeployBlockHeight uint64 `json:"DeployBlockHeight,omitempty"`

	// ContractAddr 合约地址，以太坊/Solana上需要
	ContractAddr string `json:"ContractAddr,omitempty"`

	// Abi 合约的abi json 文件路径或内容，以太坊上需要
	Abi string `json:"Abi,omitempty"`

	// GetHistoryEventInterval 轮训以太坊历史事件的间隔时间：ms
	GetHistoryEventInterval uint64 `json:"GetHistoryEventInterval,omitempty"`

	// GetHistoryEventHeightWindow 轮训以太坊历史事件的区块高度窗口大小
	GetHistoryEventHeightWindow uint64 `json:"GetHistoryEventHeightWindow,omitempty"`

	// SolanaMethods Solana 方法调用规范：methodName -> MethodSpec
	SolanaMethods map[string]SolanaMethodSpec `json:"SolanaMethods,omitempty"`
}
