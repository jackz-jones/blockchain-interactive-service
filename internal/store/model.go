package store

import (
	"time"

	"gorm.io/gorm"
)

// Tenant 租户表
type Tenant struct {
	gorm.Model
	Name   string       `gorm:"size:128;not null;uniqueIndex:idx_tenants_name" json:"name"`   // 租户名称
	Email  string       `gorm:"size:256;not null;uniqueIndex:idx_tenants_email" json:"email"` // 联系邮箱
	Phone  string       `gorm:"size:32" json:"phone"`                                         // 联系电话
	Status TenantStatus `gorm:"size:16;not null;default:active" json:"status"`                // 状态：active、disabled、suspended
	Plan   string       `gorm:"size:32;not null;default:free" json:"plan"`                    // 套餐：free、developer、enterprise
}

// TenantStatus 租户状态枚举
type TenantStatus string

const (
	TenantStatusActive    TenantStatus = "active"
	TenantStatusDisabled  TenantStatus = "disabled"
	TenantStatusSuspended TenantStatus = "suspended"
)

// User 用户表（租户下的子账号）
type User struct {
	gorm.Model
	TenantID uint     `gorm:"not null;index" json:"tenant_id"`                                 // 所属租户
	Username string   `gorm:"size:64;not null;uniqueIndex:idx_users_username" json:"username"` // 用户名
	Password string   `gorm:"size:256;not null" json:"-"`                                      // 密码哈希
	Role     UserRole `gorm:"size:16;not null;default:developer" json:"role"`                  // 角色：admin、developer、readonly
	Status   string   `gorm:"size:16;not null;default:active" json:"status"`                   // 状态

	Tenant Tenant `gorm:"foreignKey:TenantID" json:"-"`
}

// UserRole 用户角色枚举
type UserRole string

const (
	UserRoleAdmin     UserRole = "admin"
	UserRoleDeveloper UserRole = "developer"
	UserRoleReadonly  UserRole = "readonly"
)

// APIKey API 密钥表
type APIKey struct {
	gorm.Model
	TenantID    uint       `gorm:"not null;index" json:"tenant_id"`                           // 所属租户
	UserID      uint       `gorm:"not null;index" json:"user_id"`                             // 创建者
	Key         string     `gorm:"size:128;not null;uniqueIndex:idx_api_keys_key" json:"key"` // API Key 值
	Name        string     `gorm:"size:128;not null" json:"name"`                             // Key 名称/描述
	Permissions string     `gorm:"size:512" json:"permissions"`                               // 权限范围（JSON 数组）
	IPWhitelist string     `gorm:"size:1024" json:"ip_whitelist"`                             // IP 白名单（逗号分隔）
	Status      string     `gorm:"size:16;not null;default:active" json:"status"`             // 状态：active、revoked
	ExpiresAt   *time.Time `json:"expires_at"`                                                // 过期时间，nil 表示永不过期
	LastUsedAt  *time.Time `json:"last_used_at"`                                              // 最后使用时间

	Tenant Tenant `gorm:"foreignKey:TenantID" json:"-"`
	User   User   `gorm:"foreignKey:UserID" json:"-"`
}

// TenantChainConfig 租户链配置表
type TenantChainConfig struct {
	gorm.Model
	TenantID  uint   `gorm:"not null;index:idx_tenant_chain,unique" json:"tenant_id"`          // 所属租户
	ChainName string `gorm:"size:64;not null;index:idx_tenant_chain,unique" json:"chain_name"` // 链名称
	ChainType string `gorm:"size:32;not null" json:"chain_type"`                               // 链类型
	Enable    bool   `gorm:"not null;default:true" json:"enable"`                              // 是否启用

	// ========== ChainMaker 专属字段 ==========
	ChainId     string `gorm:"size:128" json:"chain_id"`       // 区块链 ID
	AuthType    string `gorm:"size:32" json:"auth_type"`       // 认证模式：public / permissionedwithcert
	OrgId       string `gorm:"size:128" json:"org_id"`         // 组织 ID（证书模式下必填）
	HashType    string `gorm:"size:32" json:"hash_type"`       // 哈希算法：SHA256 / SM3 / SHA3_256
	SignKey     string `gorm:"type:text" json:"sign_key"`      // 签名私钥密文（base64 编码）
	SignCert    string `gorm:"type:text" json:"sign_cert"`     // 签名证书密文（base64 编码，证书模式下必填）
	UserTlsKey  string `gorm:"type:text" json:"user_tls_key"`  // TLS 私钥密文（base64 编码）
	UserTlsCert string `gorm:"type:text" json:"user_tls_cert"` // TLS 证书密文（base64 编码）
	UserEncKey  string `gorm:"type:text" json:"user_enc_key"`  // 国密加密私钥密文（可选）
	UserEncCert string `gorm:"type:text" json:"user_enc_cert"` // 国密加密证书密文（可选）
	ProxyUrl    string `gorm:"size:512" json:"proxy_url"`      // 代理地址（可选，如 socks5://host:port）

	// ========== Ethereum 专属字段 ==========
	EthChainId   int64  `gorm:"default:0" json:"eth_chain_id"` // 以太坊链 ID
	HttpUrl      string `gorm:"size:512" json:"http_url"`      // 节点 HTTP URL
	WebsocketUrl string `gorm:"size:512" json:"websocket_url"` // 节点 WebSocket URL
	PrivateKey   string `gorm:"type:text" json:"private_key"`  // 私钥 hex 字符串
	GasLimit     int64  `gorm:"default:0" json:"gas_limit"`    // Gas 上限

	// ========== Solana 专属字段 ==========
	SolRpcUrl       string `gorm:"size:512" json:"sol_rpc_url"`         // Solana RPC URL
	SolPrivateKey   string `gorm:"type:text" json:"sol_private_key"`    // 私钥 base58 字符串
	CommitmentLevel string `gorm:"size:32" json:"commitment_level"`     // 确认级别
	SkipPreflight   bool   `gorm:"default:false" json:"skip_preflight"` // 是否跳过预检
	MaxRetries      int    `gorm:"default:0" json:"max_retries"`        // 最大重试次数

	Tenant Tenant `gorm:"foreignKey:TenantID" json:"-"`
}

// TenantChainNode 租户链节点配置表（ChainMaker 专用）
type TenantChainNode struct {
	gorm.Model
	ChainConfigID uint   `gorm:"not null;index" json:"chain_config_id"`    // 关联的链配置 ID
	NodeAddr      string `gorm:"size:256;not null" json:"node_addr"`       // 节点 gRPC 地址
	ConnCnt       int    `gorm:"not null;default:10" json:"conn_cnt"`      // 连接数
	EnableTls     bool   `gorm:"not null;default:false" json:"enable_tls"` // 是否开启 TLS
	TlsHostName   string `gorm:"size:256" json:"tls_host_name"`            // TLS 主机名
	CaCert        string `gorm:"type:text" json:"ca_cert"`                 // CA 证书密文（base64 编码，开启 TLS 时必填）

	ChainConfig TenantChainConfig `gorm:"foreignKey:ChainConfigID" json:"-"`
}

// TenantContractConfig 租户合约配置表
type TenantContractConfig struct {
	gorm.Model
	TenantID        uint   `gorm:"not null;index" json:"tenant_id"`                // 所属租户
	ChainConfigID   uint   `gorm:"not null;index" json:"chain_config_id"`          // 关联的链配置
	ContractName    string `gorm:"size:128;not null" json:"contract_name"`         // 合约名称
	ContractAddr    string `gorm:"size:256" json:"contract_addr"`                  // 合约地址
	AbiJSON         string `gorm:"type:text" json:"abi_json"`                      // ABI JSON
	EnableSubscribe bool   `gorm:"not null;default:false" json:"enable_subscribe"` // 是否开启事件订阅
	ExtraConf       string `gorm:"type:text" json:"extra_conf"`                    // 额外配置 JSON

	Tenant      Tenant            `gorm:"foreignKey:TenantID" json:"-"`
	ChainConfig TenantChainConfig `gorm:"foreignKey:ChainConfigID" json:"-"`
}

// CallLog 调用记录表
type CallLog struct {
	gorm.Model
	TenantID     uint   `gorm:"not null;index" json:"tenant_id"`          // 所属租户
	UserID       uint   `gorm:"not null;index" json:"user_id"`            // 调用者
	APIKeyID     uint   `gorm:"not null;index" json:"api_key_id"`         // 使用的 API Key
	ChainName    string `gorm:"size:64;not null;index" json:"chain_name"` // 链名称
	ChainType    string `gorm:"size:32;not null" json:"chain_type"`       // 链类型
	Method       string `gorm:"size:128;not null" json:"method"`          // 调用方法
	ContractName string `gorm:"size:128" json:"contract_name"`            // 合约名称
	Status       string `gorm:"size:16;not null" json:"status"`           // 调用状态：success、failed
	ErrorMsg     string `gorm:"size:1024" json:"error_msg"`               // 错误信息
	GasUsed      uint64 `gorm:"default:0" json:"gas_used"`                // Gas 消耗
	Duration     int64  `gorm:"default:0" json:"duration"`                // 耗时（毫秒）
	RequestIP    string `gorm:"size:64" json:"request_ip"`                // 请求 IP
}

// Bill 账单表
type Bill struct {
	gorm.Model
	TenantID    uint      `gorm:"not null;index" json:"tenant_id"`               // 所属租户
	PeriodStart time.Time `gorm:"not null" json:"period_start"`                  // 账期开始
	PeriodEnd   time.Time `gorm:"not null" json:"period_end"`                    // 账期结束
	TotalCalls  uint64    `gorm:"default:0" json:"total_calls"`                  // 总调用次数
	TotalGas    uint64    `gorm:"default:0" json:"total_gas"`                    // 总 Gas 消耗
	Amount      float64   `gorm:"type:decimal(10,4);default:0" json:"amount"`    // 账单金额
	Currency    string    `gorm:"size:8;not null;default:CNY" json:"currency"`   // 币种
	Status      string    `gorm:"size:16;not null;default:unpaid" json:"status"` // 状态：unpaid、paid、overdue

	Tenant Tenant `gorm:"foreignKey:TenantID" json:"-"`
}

// Quota 配额表
type Quota struct {
	gorm.Model
	TenantID      uint   `gorm:"not null;uniqueIndex:idx_quotas_tenant_id" json:"tenant_id"` // 所属租户
	MonthlyLimit  uint64 `gorm:"not null;default:1000" json:"monthly_limit"`                 // 月调用上限
	DailyLimit    uint64 `gorm:"not null;default:100" json:"daily_limit"`                    // 日调用上限
	RateLimit     int    `gorm:"not null;default:10" json:"rate_limit"`                      // QPS 限制
	MonthlyUsed   uint64 `gorm:"default:0" json:"monthly_used"`                              // 当月已用
	OveragePolicy string `gorm:"size:16;not null;default:throttle" json:"overage_policy"`    // 超额策略：throttle、block

	Tenant Tenant `gorm:"foreignKey:TenantID" json:"-"`
}

// AuditLog 审计日志表
type AuditLog struct {
	gorm.Model
	TenantID  uint   `gorm:"not null;index" json:"tenant_id"`      // 所属租户
	UserID    uint   `gorm:"not null;index" json:"user_id"`        // 操作者
	Action    string `gorm:"size:64;not null;index" json:"action"` // 操作类型
	Resource  string `gorm:"size:128" json:"resource"`             // 操作资源
	Detail    string `gorm:"type:text" json:"detail"`              // 操作详情 JSON
	IP        string `gorm:"size:64" json:"ip"`                    // 操作 IP
	UserAgent string `gorm:"size:256" json:"user_agent"`           // User-Agent
}
