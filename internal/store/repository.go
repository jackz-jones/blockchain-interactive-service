package store

import (
	"context"
	"errors"
	"time"

	"gorm.io/gorm"
)

// Repository 数据访问层接口
type Repository interface {
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
	ListChainConfigsByTenant(ctx context.Context, tenantID uint) ([]*TenantChainConfig, error)
	UpdateChainConfig(ctx context.Context, config *TenantChainConfig) error
	DeleteChainConfig(ctx context.Context, id uint) error

	// 租户合约配置相关
	CreateContractConfig(ctx context.Context, config *TenantContractConfig) error
	GetContractConfig(ctx context.Context, id uint) (*TenantContractConfig, error)
	ListContractConfigsByChain(ctx context.Context, chainConfigID uint) ([]*TenantContractConfig, error)
	UpdateContractConfig(ctx context.Context, config *TenantContractConfig) error
	DeleteContractConfig(ctx context.Context, id uint) error

	// 调用记录相关
	CreateCallLog(ctx context.Context, log *CallLog) error
	ListCallLogs(ctx context.Context, filter CallLogFilter, offset, limit int) ([]*CallLog, int64, error)
	CountCallsByTenantToday(ctx context.Context, tenantID uint) (int64, error)
	CountCallsByTenantMonth(ctx context.Context, tenantID uint, year int, month time.Month) (int64, error)

	// 账单相关
	CreateBill(ctx context.Context, bill *Bill) error
	ListBillsByTenant(ctx context.Context, tenantID uint, offset, limit int) ([]*Bill, int64, error)
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
	ChainName    string
	ContractName string
	Status       string
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

func (r *GormRepository) ListUsersByTenant(ctx context.Context, tenantID uint, offset, limit int) ([]*User, int64, error) {
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

func (r *GormRepository) ListAPIKeysByTenant(ctx context.Context, tenantID uint, offset, limit int) ([]*APIKey, int64, error) {
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

func (r *GormRepository) GetChainConfig(ctx context.Context, tenantID uint, chainName string) (*TenantChainConfig, error) {
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
	return r.db.WithContext(ctx).Save(config).Error
}

func (r *GormRepository) DeleteChainConfig(ctx context.Context, id uint) error {
	return r.db.WithContext(ctx).Delete(&TenantChainConfig{}, id).Error
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

func (r *GormRepository) ListContractConfigsByChain(ctx context.Context, chainConfigID uint) ([]*TenantContractConfig, error) {
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

// ========== 调用记录 ==========

func (r *GormRepository) CreateCallLog(ctx context.Context, log *CallLog) error {
	return r.db.WithContext(ctx).Create(log).Error
}

func (r *GormRepository) ListCallLogs(ctx context.Context, filter CallLogFilter, offset, limit int) ([]*CallLog, int64, error) {
	var logs []*CallLog
	var total int64
	db := r.db.WithContext(ctx).Model(&CallLog{})

	if filter.TenantID > 0 {
		db = db.Where("tenant_id = ?", filter.TenantID)
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
		Where("tenant_id = ? AND created_at >= ?", tenantID, todayStart).
		Count(&count).Error
	return count, err
}

func (r *GormRepository) CountCallsByTenantMonth(ctx context.Context, tenantID uint, year int, month time.Month) (int64, error) {
	var count int64
	monthStart := time.Date(year, month, 1, 0, 0, 0, 0, time.Local)
	monthEnd := monthStart.AddDate(0, 1, 0)
	err := r.db.WithContext(ctx).Model(&CallLog{}).
		Where("tenant_id = ? AND created_at >= ? AND created_at < ?", tenantID, monthStart, monthEnd).
		Count(&count).Error
	return count, err
}

// ========== 账单 ==========

func (r *GormRepository) CreateBill(ctx context.Context, bill *Bill) error {
	return r.db.WithContext(ctx).Create(bill).Error
}

func (r *GormRepository) ListBillsByTenant(ctx context.Context, tenantID uint, offset, limit int) ([]*Bill, int64, error) {
	var bills []*Bill
	var total int64
	db := r.db.WithContext(ctx).Model(&Bill{}).Where("tenant_id = ?", tenantID)
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

func (r *GormRepository) ListAuditLogs(ctx context.Context, filter AuditLogFilter, offset, limit int) ([]*AuditLog, int64, error) {
	var logs []*AuditLog
	var total int64
	db := r.db.WithContext(ctx).Model(&AuditLog{})

	if filter.TenantID > 0 {
		db = db.Where("tenant_id = ?", filter.TenantID)
	}
	if filter.UserID > 0 {
		db = db.Where("user_id = ?", filter.UserID)
	}
	if filter.Action != "" {
		db = db.Where("action = ?", filter.Action)
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
