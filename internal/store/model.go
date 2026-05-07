package store

import (
	"time"

	"gorm.io/gorm"
)

// Tenant 租户表
type Tenant struct {
	ID        uint           `gorm:"primaryKey" json:"id"`
	Name      string         `gorm:"size:128;not null;uniqueIndex" json:"name"`        // 租户名称
	Email     string         `gorm:"size:256;not null;uniqueIndex" json:"email"`        // 联系邮箱
	Phone     string         `gorm:"size:32" json:"phone"`                             // 联系电话
	Status    TenantStatus   `gorm:"size:16;not null;default:active" json:"status"`    // 状态：active、disabled、suspended
	Plan      string         `gorm:"size:32;not null;default:free" json:"plan"`        // 套餐：free、developer、enterprise
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
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
	ID        uint           `gorm:"primaryKey" json:"id"`
	TenantID  uint           `gorm:"not null;index" json:"tenant_id"`                  // 所属租户
	Username  string         `gorm:"size:64;not null;uniqueIndex" json:"username"`     // 用户名
	Password  string         `gorm:"size:256;not null" json:"-"`                       // 密码哈希
	Role      UserRole       `gorm:"size:16;not null;default:developer" json:"role"`   // 角色：admin、developer、readonly
	Status    string         `gorm:"size:16;not null;default:active" json:"status"`    // 状态
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`

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
	ID          uint           `gorm:"primaryKey" json:"id"`
	TenantID    uint           `gorm:"not null;index" json:"tenant_id"`                   // 所属租户
	UserID      uint           `gorm:"not null;index" json:"user_id"`                     // 创建者
	Key         string         `gorm:"size:64;not null;uniqueIndex" json:"key"`           // API Key 值
	Name        string         `gorm:"size:128;not null" json:"name"`                     // Key 名称/描述
	Permissions string         `gorm:"size:512" json:"permissions"`                       // 权限范围（JSON 数组）
	IPWhitelist string         `gorm:"size:1024" json:"ip_whitelist"`                     // IP 白名单（逗号分隔）
	Status      string         `gorm:"size:16;not null;default:active" json:"status"`     // 状态：active、revoked
	ExpiresAt   *time.Time     `json:"expires_at"`                                        // 过期时间，nil 表示永不过期
	LastUsedAt  *time.Time     `json:"last_used_at"`                                      // 最后使用时间
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
	DeletedAt   gorm.DeletedAt `gorm:"index" json:"-"`

	Tenant Tenant `gorm:"foreignKey:TenantID" json:"-"`
	User   User   `gorm:"foreignKey:UserID" json:"-"`
}

// TenantChainConfig 租户链配置表
type TenantChainConfig struct {
	ID        uint           `gorm:"primaryKey" json:"id"`
	TenantID  uint           `gorm:"not null;index:idx_tenant_chain,unique" json:"tenant_id"` // 所属租户
	ChainName string         `gorm:"size:64;not null;index:idx_tenant_chain,unique" json:"chain_name"` // 链名称
	ChainType string         `gorm:"size:32;not null" json:"chain_type"`                      // 链类型：ethereum、chainmaker、solana
	Enable    bool           `gorm:"not null;default:true" json:"enable"`                     // 是否启用
	SdkConf   string         `gorm:"type:text" json:"sdk_conf"`                              // SDK 配置 JSON
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`

	Tenant Tenant `gorm:"foreignKey:TenantID" json:"-"`
}

// TenantContractConfig 租户合约配置表
type TenantContractConfig struct {
	ID           uint           `gorm:"primaryKey" json:"id"`
	TenantID     uint           `gorm:"not null;index" json:"tenant_id"`                       // 所属租户
	ChainConfigID uint          `gorm:"not null;index" json:"chain_config_id"`                 // 关联的链配置
	ContractName string         `gorm:"size:128;not null" json:"contract_name"`                // 合约名称
	ContractAddr string         `gorm:"size:256" json:"contract_addr"`                         // 合约地址
	ContractType string         `gorm:"size:32" json:"contract_type"`                          // 合约类型
	AbiJSON      string         `gorm:"type:text" json:"abi_json"`                             // ABI JSON
	ExtraConf    string         `gorm:"type:text" json:"extra_conf"`                           // 额外配置 JSON
	CreatedAt    time.Time      `json:"created_at"`
	UpdatedAt    time.Time      `json:"updated_at"`
	DeletedAt    gorm.DeletedAt `gorm:"index" json:"-"`

	Tenant      Tenant            `gorm:"foreignKey:TenantID" json:"-"`
	ChainConfig TenantChainConfig `gorm:"foreignKey:ChainConfigID" json:"-"`
}

// CallLog 调用记录表
type CallLog struct {
	ID         uint      `gorm:"primaryKey" json:"id"`
	TenantID   uint      `gorm:"not null;index:idx_tenant_time" json:"tenant_id"`       // 所属租户
	UserID     uint      `gorm:"not null;index" json:"user_id"`                         // 调用者
	APIKeyID   uint      `gorm:"not null;index" json:"api_key_id"`                      // 使用的 API Key
	ChainName  string    `gorm:"size:64;not null;index" json:"chain_name"`              // 链名称
	ChainType  string    `gorm:"size:32;not null" json:"chain_type"`                    // 链类型
	Method     string    `gorm:"size:128;not null" json:"method"`                       // 调用方法
	ContractName string  `gorm:"size:128" json:"contract_name"`                         // 合约名称
	Status     string    `gorm:"size:16;not null" json:"status"`                        // 调用状态：success、failed
	ErrorMsg   string    `gorm:"size:1024" json:"error_msg"`                            // 错误信息
	GasUsed    uint64    `gorm:"default:0" json:"gas_used"`                             // Gas 消耗
	Duration   int64     `gorm:"default:0" json:"duration"`                             // 耗时（毫秒）
	RequestIP  string    `gorm:"size:64" json:"request_ip"`                             // 请求 IP
	CreatedAt  time.Time `gorm:"index:idx_tenant_time" json:"created_at"`               // 调用时间
}

// Bill 账单表
type Bill struct {
	ID          uint      `gorm:"primaryKey" json:"id"`
	TenantID    uint      `gorm:"not null;index" json:"tenant_id"`                      // 所属租户
	PeriodStart time.Time `gorm:"not null" json:"period_start"`                         // 账期开始
	PeriodEnd   time.Time `gorm:"not null" json:"period_end"`                           // 账期结束
	TotalCalls  uint64    `gorm:"default:0" json:"total_calls"`                         // 总调用次数
	TotalGas    uint64    `gorm:"default:0" json:"total_gas"`                           // 总 Gas 消耗
	Amount      float64   `gorm:"type:decimal(10,4);default:0" json:"amount"`           // 账单金额
	Currency    string    `gorm:"size:8;not null;default:CNY" json:"currency"`          // 币种
	Status      string    `gorm:"size:16;not null;default:unpaid" json:"status"`        // 状态：unpaid、paid、overdue
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`

	Tenant Tenant `gorm:"foreignKey:TenantID" json:"-"`
}

// Quota 配额表
type Quota struct {
	ID             uint      `gorm:"primaryKey" json:"id"`
	TenantID       uint      `gorm:"not null;uniqueIndex" json:"tenant_id"`              // 所属租户
	MonthlyLimit   uint64    `gorm:"not null;default:1000" json:"monthly_limit"`         // 月调用上限
	DailyLimit     uint64    `gorm:"not null;default:100" json:"daily_limit"`            // 日调用上限
	RateLimit      int       `gorm:"not null;default:10" json:"rate_limit"`              // QPS 限制
	MonthlyUsed    uint64    `gorm:"default:0" json:"monthly_used"`                      // 当月已用
	OveragePolicy  string    `gorm:"size:16;not null;default:throttle" json:"overage_policy"` // 超额策略：throttle、block
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`

	Tenant Tenant `gorm:"foreignKey:TenantID" json:"-"`
}

// AuditLog 审计日志表
type AuditLog struct {
	ID         uint      `gorm:"primaryKey" json:"id"`
	TenantID   uint      `gorm:"not null;index:idx_audit_tenant_time" json:"tenant_id"` // 所属租户
	UserID     uint      `gorm:"not null;index" json:"user_id"`                         // 操作者
	Action     string    `gorm:"size:64;not null;index" json:"action"`                  // 操作类型
	Resource   string    `gorm:"size:128" json:"resource"`                              // 操作资源
	Detail     string    `gorm:"type:text" json:"detail"`                               // 操作详情 JSON
	IP         string    `gorm:"size:64" json:"ip"`                                     // 操作 IP
	UserAgent  string    `gorm:"size:256" json:"user_agent"`                            // User-Agent
	CreatedAt  time.Time `gorm:"index:idx_audit_tenant_time" json:"created_at"`         // 操作时间
}
