package store

import (
	"context"
	"errors"
	"time"

	"gorm.io/gorm"
)

// Repository 数据访问层接口
type Repository interface {
	// 获取底层数据库连接（用于事务操作）
	DB() *gorm.DB

	// 租户相关
	CreateTenant(ctx context.Context, tenant *Tenant) error
	GetTenantByID(ctx context.Context, id uint) (*Tenant, error)
	GetTenantByName(ctx context.Context, name string) (*Tenant, error)
	UpdateTenant(ctx context.Context, tenant *Tenant) error
	ListTenants(ctx context.Context, offset, limit int) ([]*Tenant, int64, error)

	// 用户相关
	CreateUser(ctx context.Context, user *User) error
	GetUserByID(ctx context.Context, id uint) (*User, error)
	GetUserByUsername(ctx context.Context, username string) (*User, error)
	ListUsersByTenant(ctx context.Context, tenantID uint, offset, limit int) ([]*User, int64, error)
	UpdateUser(ctx context.Context, user *User) error

	// API Key 相关
	CreateAPIKey(ctx context.Context, apiKey *APIKey) error
	GetAPIKeyByKey(ctx context.Context, key string) (*APIKey, error)
	ListAPIKeysByTenant(ctx context.Context, tenantID uint, offset, limit int) ([]*APIKey, int64, error)
	UpdateAPIKey(ctx context.Context, apiKey *APIKey) error
	UpdateAPIKeyLastUsed(ctx context.Context, id uint, t time.Time) error

	// 租户链配置相关
	CreateChainConfig(ctx context.Context, config *TenantChainConfig) error
	GetChainConfig(ctx context.Context, tenantID uint, chainName string) (*TenantChainConfig, error)
	GetChainConfigByID(ctx context.Context, id uint) (*TenantChainConfig, error)
	ListChainConfigsByTenant(ctx context.Context, tenantID uint) ([]*TenantChainConfig, error)
	UpdateChainConfig(ctx context.Context, config *TenantChainConfig) error
	DeleteChainConfig(ctx context.Context, id uint) error
	CheckChainNameUnique(ctx context.Context, tenantID uint, chainName string, excludeID uint) (bool, error)

	// 租户链节点配置相关
	CreateChainNodes(ctx context.Context, nodes []*TenantChainNode) error
	ListChainNodesByConfigID(ctx context.Context, chainConfigID uint) ([]*TenantChainNode, error)
	DeleteChainNodesByConfigID(ctx context.Context, chainConfigID uint) error
	ReplaceChainNodes(ctx context.Context, chainConfigID uint, nodes []*TenantChainNode) error

	// 租户合约配置相关
	CreateContractConfig(ctx context.Context, config *TenantContractConfig) error
	GetContractConfig(ctx context.Context, id uint) (*TenantContractConfig, error)
	ListContractConfigsByChain(ctx context.Context, chainConfigID uint) ([]*TenantContractConfig, error)
	UpdateContractConfig(ctx context.Context, config *TenantContractConfig) error
	DeleteContractConfig(ctx context.Context, id uint) error
	CheckContractNameUnique(ctx context.Context, chainConfigID uint, contractName string, excludeID uint) (bool, error)
	ListEnabledSubscribeContracts(ctx context.Context, tenantID uint) ([]*TenantContractConfig, error)
	ListDisabledSubscribeContracts(ctx context.Context, tenantID uint) ([]*TenantContractConfig, error)
	ListAllEnabledSubscribeContracts(ctx context.Context) ([]*TenantContractConfig, error)
	GetContractConfigByChainAndName(
		ctx context.Context, tenantID uint, chainName, contractName string,
	) (*TenantContractConfig, error)
	GetContractConfigByChainAndAddr(
		ctx context.Context, tenantID uint, chainName, contractAddr string,
	) (*TenantContractConfig, error)

	// 调用记录相关
	CreateCallLog(ctx context.Context, log *CallLog) error
	ListCallLogs(ctx context.Context, filter CallLogFilter, offset, limit int) ([]*CallLog, int64, error)
	CountCallsByTenantToday(ctx context.Context, tenantID uint) (int64, error)
	CountCallsByTenantMonth(ctx context.Context, tenantID uint, year int, month time.Month) (int64, error)
	CountCallsByTenantDay(ctx context.Context, tenantID uint, year int, month time.Month, day int) (int64, error)
	GetDailyUsageStats(ctx context.Context, tenantID uint, startTime, endTime time.Time) ([]*DailyUsageStat, error)
	GetDailyUsageStatsByMethodType(ctx context.Context, tenantID uint, startTime, endTime time.Time) ([]*DailyUsageStat, error)

	// 账单相关
	CreateBill(ctx context.Context, bill *Bill) error
	ListBillsByTenant(ctx context.Context, tenantID uint, billType string, offset, limit int) ([]*Bill, int64, error)
	UpdateBill(ctx context.Context, bill *Bill) error

	// 配额相关
	GetQuotaByTenant(ctx context.Context, tenantID uint) (*Quota, error)
	CreateOrUpdateQuota(ctx context.Context, quota *Quota) error
	IncrementMonthlyUsed(ctx context.Context, tenantID uint, delta uint64) error

	// 审计日志相关
	CreateAuditLog(ctx context.Context, log *AuditLog) error
	ListAuditLogs(ctx context.Context, filter AuditLogFilter, offset, limit int) ([]*AuditLog, int64, error)
}

// CallLogFilter 调用记录查询过滤条件
type CallLogFilter struct {
	TenantID     uint
	UserID       uint
	ChainName    string
	ContractName string
	Status       string
	MethodType   MethodType
	StartTime    *time.Time
	EndTime      *time.Time
}

// AuditLogFilter 审计日志查询过滤条件
type AuditLogFilter struct {
	TenantID  uint
	UserID    uint
	Action    string
	StartTime *time.Time
	EndTime   *time.Time
}

// ---- 以下为 Repository 的 GORM 实现 ----

// GormRepository 基于 GORM 的 Repository 实现
type GormRepository struct {
	db *gorm.DB
}

// NewGormRepository 创建 GormRepository 实例
func NewGormRepository(db *gorm.DB) Repository {
	return &GormRepository{db: db}
}

// DB 获取底层数据库连接（用于事务操作）
func (r *GormRepository) DB() *gorm.DB {
	return r.db
}

// ========== 租户 ==========

func (r *GormRepository) CreateTenant(ctx context.Context, tenant *Tenant) error {
	return r.db.WithContext(ctx).Create(tenant).Error
}

func (r *GormRepository) GetTenantByID(ctx context.Context, id uint) (*Tenant, error) {
	var tenant Tenant
	err := r.db.WithContext(ctx).First(&tenant, id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	return &tenant, err
}

func (r *GormRepository) GetTenantByName(ctx context.Context, name string) (*Tenant, error) {
	var tenant Tenant
	err := r.db.WithContext(ctx).Where("name = ?", name).First(&tenant).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	return &tenant, err
}

func (r *GormRepository) UpdateTenant(ctx context.Context, tenant *Tenant) error {
	return r.db.WithContext(ctx).Save(tenant).Error
}

func (r *GormRepository) ListTenants(ctx context.Context, offset, limit int) ([]*Tenant, int64, error) {
	var tenants []*Tenant
	var total int64
	db := r.db.WithContext(ctx).Model(&Tenant{})
	if err := db.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	if err := db.Offset(offset).Limit(limit).Order("id DESC").Find(&tenants).Error; err != nil {
		return nil, 0, err
	}
	return tenants, total, nil
}

// ========== 用户 ==========

func (r *GormRepository) CreateUser(ctx context.Context, user *User) error {
	return r.db.WithContext(ctx).Create(user).Error
}

func (r *GormRepository) GetUserByID(ctx context.Context, id uint) (*User, error) {
	var user User
	err := r.db.WithContext(ctx).First(&user, id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	return &user, err
}

func (r *GormRepository) GetUserByUsername(ctx context.Context, username string) (*User, error) {
	var user User
	err := r.db.WithContext(ctx).Where("username = ?", username).First(&user).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	return &user, err
}

func (r *GormRepository) ListUsersByTenant(
	ctx context.Context, tenantID uint, offset, limit int,
) ([]*User, int64, error) {
	var users []*User
	var total int64
	db := r.db.WithContext(ctx).Model(&User{}).Where("tenant_id = ?", tenantID)
	if err := db.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	if err := db.Offset(offset).Limit(limit).Order("id DESC").Find(&users).Error; err != nil {
		return nil, 0, err
	}
	return users, total, nil
}

func (r *GormRepository) UpdateUser(ctx context.Context, user *User) error {
	return r.db.WithContext(ctx).Save(user).Error
}

// ========== API Key ==========

func (r *GormRepository) CreateAPIKey(ctx context.Context, apiKey *APIKey) error {
	return r.db.WithContext(ctx).Create(apiKey).Error
}

func (r *GormRepository) GetAPIKeyByKey(ctx context.Context, key string) (*APIKey, error) {
	var apiKey APIKey
	err := r.db.WithContext(ctx).Where("`key` = ?", key).First(&apiKey).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	return &apiKey, err
}

func (r *GormRepository) ListAPIKeysByTenant(
	ctx context.Context, tenantID uint, offset, limit int,
) ([]*APIKey, int64, error) {
	var keys []*APIKey
	var total int64
	db := r.db.WithContext(ctx).Model(&APIKey{}).Where("tenant_id = ?", tenantID)
	if err := db.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	if err := db.Offset(offset).Limit(limit).Order("id DESC").Find(&keys).Error; err != nil {
		return nil, 0, err
	}
	return keys, total, nil
}

func (r *GormRepository) UpdateAPIKey(ctx context.Context, apiKey *APIKey) error {
	return r.db.WithContext(ctx).Save(apiKey).Error
}

func (r *GormRepository) UpdateAPIKeyLastUsed(ctx context.Context, id uint, t time.Time) error {
	return r.db.WithContext(ctx).Model(&APIKey{}).Where("id = ?", id).Update("last_used_at", t).Error
}

// ========== 租户链配置 ==========

func (r *GormRepository) CreateChainConfig(ctx context.Context, config *TenantChainConfig) error {
	return r.db.WithContext(ctx).Create(config).Error
}

func (r *GormRepository) GetChainConfig(
	ctx context.Context, tenantID uint, chainName string,
) (*TenantChainConfig, error) {
	var config TenantChainConfig
	err := r.db.WithContext(ctx).Where("tenant_id = ? AND chain_name = ?", tenantID, chainName).First(&config).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	return &config, err
}

func (r *GormRepository) ListChainConfigsByTenant(ctx context.Context, tenantID uint) ([]*TenantChainConfig, error) {
	var configs []*TenantChainConfig
	err := r.db.WithContext(ctx).Where("tenant_id = ?", tenantID).Find(&configs).Error
	return configs, err
}

func (r *GormRepository) UpdateChainConfig(ctx context.Context, config *TenantChainConfig) error {
	// 使用 Select 指定要更新的字段，避免 Save() 覆盖 created_at 等不应被修改的字段
	return r.db.WithContext(ctx).Model(config).Select(
		"chain_name", "chain_type", "enable",
		"chain_id", "auth_type", "org_id", "hash_type",
		"sign_key", "sign_cert", "user_tls_key", "user_tls_cert",
		"user_enc_key", "user_enc_cert", "proxy_url",
		"eth_chain_id", "http_url", "websocket_url", "private_key",
		"sol_rpc_url", "sol_private_key", "commitment_level", "skip_preflight", "max_retries",
	).Updates(config).Error
}

func (r *GormRepository) DeleteChainConfig(ctx context.Context, id uint) error {
	return r.db.WithContext(ctx).Delete(&TenantChainConfig{}, id).Error
}

func (r *GormRepository) GetChainConfigByID(ctx context.Context, id uint) (*TenantChainConfig, error) {
	var config TenantChainConfig
	err := r.db.WithContext(ctx).First(&config, id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	return &config, err
}

func (r *GormRepository) CheckChainNameUnique(
	ctx context.Context, tenantID uint, chainName string, excludeID uint,
) (bool, error) {
	var count int64
	db := r.db.WithContext(ctx).Model(&TenantChainConfig{}).Where("tenant_id = ? AND chain_name = ?", tenantID, chainName)
	if excludeID > 0 {
		db = db.Where("id != ?", excludeID)
	}
	if err := db.Count(&count).Error; err != nil {
		return false, err
	}
	return count == 0, nil
}

// ========== 租户链节点配置 ==========

func (r *GormRepository) CreateChainNodes(ctx context.Context, nodes []*TenantChainNode) error {
	if len(nodes) == 0 {
		return nil
	}
	return r.db.WithContext(ctx).Create(&nodes).Error
}

func (r *GormRepository) ListChainNodesByConfigID(ctx context.Context, chainConfigID uint) ([]*TenantChainNode, error) {
	var nodes []*TenantChainNode
	err := r.db.WithContext(ctx).Where("chain_config_id = ?", chainConfigID).Find(&nodes).Error
	return nodes, err
}

func (r *GormRepository) DeleteChainNodesByConfigID(ctx context.Context, chainConfigID uint) error {
	return r.db.WithContext(ctx).Where("chain_config_id = ?", chainConfigID).Delete(&TenantChainNode{}).Error
}

// ReplaceChainNodes 在事务中全量替换指定链配置的节点列表
func (r *GormRepository) ReplaceChainNodes(ctx context.Context, chainConfigID uint, nodes []*TenantChainNode) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// 删除旧节点
		if err := tx.Where("chain_config_id = ?", chainConfigID).Delete(&TenantChainNode{}).Error; err != nil {
			return err
		}
		// 插入新节点
		if len(nodes) > 0 {
			for i := range nodes {
				nodes[i].ChainConfigID = chainConfigID
				nodes[i].ID = 0 // 确保是新建
			}
			if err := tx.Create(&nodes).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

// ========== 租户合约配置 ==========

func (r *GormRepository) CreateContractConfig(ctx context.Context, config *TenantContractConfig) error {
	return r.db.WithContext(ctx).Create(config).Error
}

func (r *GormRepository) GetContractConfig(ctx context.Context, id uint) (*TenantContractConfig, error) {
	var config TenantContractConfig
	err := r.db.WithContext(ctx).First(&config, id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	return &config, err
}

func (r *GormRepository) ListContractConfigsByChain(
	ctx context.Context, chainConfigID uint,
) ([]*TenantContractConfig, error) {
	var configs []*TenantContractConfig
	err := r.db.WithContext(ctx).Where("chain_config_id = ?", chainConfigID).Find(&configs).Error
	return configs, err
}

func (r *GormRepository) UpdateContractConfig(ctx context.Context, config *TenantContractConfig) error {
	return r.db.WithContext(ctx).Save(config).Error
}

func (r *GormRepository) DeleteContractConfig(ctx context.Context, id uint) error {
	return r.db.WithContext(ctx).Delete(&TenantContractConfig{}, id).Error
}

func (r *GormRepository) CheckContractNameUnique(
	ctx context.Context, chainConfigID uint, contractName string, excludeID uint,
) (bool, error) {
	var count int64
	db := r.db.WithContext(ctx).Model(&TenantContractConfig{}).
		Where("chain_config_id = ? AND contract_name = ?", chainConfigID, contractName)
	if excludeID > 0 {
		db = db.Where("id != ?", excludeID)
	}
	if err := db.Count(&count).Error; err != nil {
		return false, err
	}
	return count == 0, nil
}

func (r *GormRepository) ListEnabledSubscribeContracts(
	ctx context.Context, tenantID uint,
) ([]*TenantContractConfig, error) {
	var configs []*TenantContractConfig
	db := r.db.WithContext(ctx).Where("enable_subscribe = ?", true)
	if tenantID > 0 {
		db = db.Where("tenant_id = ?", tenantID)
	}
	err := db.Preload("ChainConfig").Find(&configs).Error
	return configs, err
}

func (r *GormRepository) ListDisabledSubscribeContracts(
	ctx context.Context, tenantID uint,
) ([]*TenantContractConfig, error) {
	var configs []*TenantContractConfig
	db := r.db.WithContext(ctx).Where("enable_subscribe = ?", false)
	if tenantID > 0 {
		db = db.Where("tenant_id = ?", tenantID)
	}
	err := db.Preload("ChainConfig").Find(&configs).Error
	return configs, err
}

func (r *GormRepository) ListAllEnabledSubscribeContracts(ctx context.Context) ([]*TenantContractConfig, error) {
	var configs []*TenantContractConfig
	err := r.db.WithContext(ctx).
		Where("enable_subscribe = ?", true).
		Preload("ChainConfig").
		Find(&configs).Error
	return configs, err
}

func (r *GormRepository) GetContractConfigByChainAndName(
	ctx context.Context, tenantID uint, chainName, contractName string,
) (*TenantContractConfig, error) {
	var config TenantContractConfig
	err := r.db.WithContext(ctx).
		Joins("JOIN tenant_chain_configs ON tenant_chain_configs.id = tenant_contract_configs.chain_config_id").
		Where(
			"tenant_contract_configs.tenant_id = ? AND tenant_chain_configs.chain_name = ?"+
				" AND tenant_contract_configs.contract_name = ?",
			tenantID, chainName, contractName).
		First(&config).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	return &config, err
}

func (r *GormRepository) GetContractConfigByChainAndAddr(
	ctx context.Context, tenantID uint, chainName, contractAddr string,
) (*TenantContractConfig, error) {
	var config TenantContractConfig
	err := r.db.WithContext(ctx).
		Joins("JOIN tenant_chain_configs ON tenant_chain_configs.id = tenant_contract_configs.chain_config_id").
		Where(
			"tenant_contract_configs.tenant_id = ? AND tenant_chain_configs.chain_name = ?"+
				" AND tenant_contract_configs.contract_addr = ?",
			tenantID, chainName, contractAddr).
		First(&config).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	return &config, err
}

// ========== 调用记录 ==========

func (r *GormRepository) CreateCallLog(ctx context.Context, log *CallLog) error {
	return r.db.WithContext(ctx).Create(log).Error
}

func (r *GormRepository) ListCallLogs(
	ctx context.Context, filter CallLogFilter, offset, limit int,
) ([]*CallLog, int64, error) {
	var logs []*CallLog
	var total int64
	db := r.db.WithContext(ctx).Model(&CallLog{})

	if filter.TenantID > 0 {
		db = db.Where("tenant_id = ?", filter.TenantID)
	}
	if filter.UserID > 0 {
		db = db.Where("user_id = ?", filter.UserID)
	}
	if filter.ChainName != "" {
		db = db.Where("chain_name = ?", filter.ChainName)
	}
	if filter.ContractName != "" {
		db = db.Where("contract_name = ?", filter.ContractName)
	}
	if filter.Status != "" {
		db = db.Where("status = ?", filter.Status)
	}
	if filter.MethodType > 0 {
		db = db.Where("method_type = ?", filter.MethodType)
	}
	if filter.StartTime != nil {
		db = db.Where("created_at >= ?", *filter.StartTime)
	}
	if filter.EndTime != nil {
		db = db.Where("created_at <= ?", *filter.EndTime)
	}

	if err := db.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	if err := db.Offset(offset).Limit(limit).Order("id DESC").Find(&logs).Error; err != nil {
		return nil, 0, err
	}
	return logs, total, nil
}

func (r *GormRepository) CountCallsByTenantToday(ctx context.Context, tenantID uint) (int64, error) {
	var count int64
	now := time.Now()
	todayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	err := r.db.WithContext(ctx).Model(&CallLog{}).
		Where("tenant_id = ? AND method_type = ? AND created_at >= ?", tenantID, MethodTypeInvoke, todayStart).
		Count(&count).Error
	return count, err
}

func (r *GormRepository) CountCallsByTenantMonth(
	ctx context.Context, tenantID uint, year int, month time.Month,
) (int64, error) {
	var count int64
	monthStart := time.Date(year, month, 1, 0, 0, 0, 0, time.Local)
	monthEnd := monthStart.AddDate(0, 1, 0)
	err := r.db.WithContext(ctx).Model(&CallLog{}).
		Where("tenant_id = ? AND method_type = ? AND created_at >= ? AND created_at < ?",
			tenantID, MethodTypeInvoke, monthStart, monthEnd).
		Count(&count).Error
	return count, err
}

func (r *GormRepository) CountCallsByTenantDay(
	ctx context.Context, tenantID uint, year int, month time.Month, day int,
) (int64, error) {
	var count int64
	dayStart := time.Date(year, month, day, 0, 0, 0, 0, time.Local)
	dayEnd := dayStart.AddDate(0, 0, 1)
	err := r.db.WithContext(ctx).Model(&CallLog{}).
		Where("tenant_id = ? AND method_type = ? AND created_at >= ? AND created_at < ?",
			tenantID, MethodTypeInvoke, dayStart, dayEnd).
		Count(&count).Error
	return count, err
}

func (r *GormRepository) GetDailyUsageStats(
	ctx context.Context, tenantID uint, startTime, endTime time.Time,
) ([]*DailyUsageStat, error) {
	var results []DailyUsageStat
	sql := `SELECT DATE(created_at) AS date, COUNT(*) AS total, ` +
		`SUM(CASE WHEN status = 'success' THEN 1 ELSE 0 END) AS success, ` +
		`SUM(CASE WHEN status = 'failed' THEN 1 ELSE 0 END) AS failed ` +
		`FROM call_logs ` +
		`WHERE tenant_id = ? AND created_at >= ? AND created_at < ? ` +
		`GROUP BY DATE(created_at) ` +
		`ORDER BY date ASC`
	err := r.db.WithContext(ctx).Raw(sql, tenantID, startTime, endTime).Scan(&results).Error
	if err != nil {
		return nil, err
	}
	out := make([]*DailyUsageStat, len(results))
	for i := range results {
		out[i] = &results[i]
	}
	return out, nil
}

// GetDailyUsageStatsByMethodType 按调用类型（Invoke/Query）分别统计每日用量
func (r *GormRepository) GetDailyUsageStatsByMethodType(
	ctx context.Context, tenantID uint, startTime, endTime time.Time,
) ([]*DailyUsageStat, error) {
	sql := `SELECT DATE(created_at) AS date, COUNT(*) AS total, ` +
		`SUM(CASE WHEN status = 'success' THEN 1 ELSE 0 END) AS success, ` +
		`SUM(CASE WHEN status = 'failed' THEN 1 ELSE 0 END) AS failed ` +
		`FROM call_logs ` +
		`WHERE tenant_id = ? AND method_type = ? AND created_at >= ? AND created_at < ? ` +
		`GROUP BY DATE(created_at) ` +
		`ORDER BY date ASC`

	// 统计 Invoke 调用
	var invokeResults []DailyUsageStat
	if err := r.db.WithContext(ctx).Raw(sql, tenantID, MethodTypeInvoke, startTime, endTime).Scan(&invokeResults).Error; err != nil {
		return nil, err
	}
	out := make([]*DailyUsageStat, len(invokeResults))
	for i := range invokeResults {
		out[i] = &invokeResults[i]
	}
	return out, nil
}

// ========== 账单 ==========

func (r *GormRepository) CreateBill(ctx context.Context, bill *Bill) error {
	return r.db.WithContext(ctx).Create(bill).Error
}

func (r *GormRepository) ListBillsByTenant(
	ctx context.Context, tenantID uint, billType string, offset, limit int,
) ([]*Bill, int64, error) {
	var bills []*Bill
	var total int64
	db := r.db.WithContext(ctx).Model(&Bill{}).Where("tenant_id = ?", tenantID)
	if billType != "" {
		db = db.Where("bill_type = ?", billType)
	}
	if err := db.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	if err := db.Offset(offset).Limit(limit).Order("id DESC").Find(&bills).Error; err != nil {
		return nil, 0, err
	}
	return bills, total, nil
}

func (r *GormRepository) UpdateBill(ctx context.Context, bill *Bill) error {
	return r.db.WithContext(ctx).Save(bill).Error
}

// ========== 配额 ==========

func (r *GormRepository) GetQuotaByTenant(ctx context.Context, tenantID uint) (*Quota, error) {
	var quota Quota
	err := r.db.WithContext(ctx).Where("tenant_id = ?", tenantID).First(&quota).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	return &quota, err
}

func (r *GormRepository) CreateOrUpdateQuota(ctx context.Context, quota *Quota) error {
	return r.db.WithContext(ctx).Save(quota).Error
}

func (r *GormRepository) IncrementMonthlyUsed(ctx context.Context, tenantID uint, delta uint64) error {
	return r.db.WithContext(ctx).Model(&Quota{}).
		Where("tenant_id = ?", tenantID).
		Update("monthly_used", gorm.Expr("monthly_used + ?", delta)).Error
}

// ========== 审计日志 ==========

func (r *GormRepository) CreateAuditLog(ctx context.Context, log *AuditLog) error {
	return r.db.WithContext(ctx).Create(log).Error
}

func (r *GormRepository) ListAuditLogs(
	ctx context.Context, filter AuditLogFilter, offset, limit int,
) ([]*AuditLog, int64, error) {
	var logs []*AuditLog
	var total int64
	db := r.db.WithContext(ctx).Model(&AuditLog{}).Joins("LEFT JOIN users ON users.id = audit_logs.user_id")

	if filter.TenantID > 0 {
		db = db.Where("audit_logs.tenant_id = ?", filter.TenantID)
	}
	if filter.UserID > 0 {
		db = db.Where("audit_logs.user_id = ?", filter.UserID)
	}
	if filter.Action != "" {
		db = db.Where("audit_logs.action = ?", filter.Action)
	}
	if filter.StartTime != nil {
		db = db.Where("audit_logs.created_at >= ?", *filter.StartTime)
	}
	if filter.EndTime != nil {
		db = db.Where("audit_logs.created_at <= ?", *filter.EndTime)
	}

	if err := db.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	if err := db.Offset(offset).Limit(limit).Order("audit_logs.id DESC").
		Find(&logs).Error; err != nil {
		return nil, 0, err
	}

	// 手动填充 operator 字段（避免 GORM 别名扫描问题）
	if len(logs) > 0 {
		userIDs := make([]uint, 0, len(logs))
		for _, log := range logs {
			if log.UserID > 0 {
				userIDs = append(userIDs, log.UserID)
			}
		}
		if len(userIDs) > 0 {
			var users []User
			r.db.WithContext(ctx).Select("id, username").Where("id IN ?", userIDs).Find(&users)
			userMap := make(map[uint]string, len(users))
			for _, u := range users {
				userMap[u.ID] = u.Username
			}
			for _, log := range logs {
				if name, ok := userMap[log.UserID]; ok {
					log.Operator = name
				}
			}
		}
	}

	return logs, total, nil
}
