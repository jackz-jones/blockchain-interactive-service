package tenant

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"github.com/jackz-jones/blockchain-interactive-service/internal/store"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

var (
	ErrTenantExists    = errors.New("tenant already exists")
	ErrTenantNotFound  = errors.New("tenant not found")
	ErrUserExists      = errors.New("user already exists")
	ErrUserNotFound    = errors.New("user not found")
	ErrInvalidPassword = errors.New("invalid password")
	ErrTenantDisabled  = errors.New("tenant is disabled or suspended")
)

// Service 租户管理服务
type Service struct {
	repo store.Repository
}

// NewService 创建租户管理服务
func NewService(repo store.Repository) *Service {
	return &Service{repo: repo}
}

// CreateTenantRequest 创建租户请求
type CreateTenantRequest struct {
	Name     string
	Email    string
	Phone    string
	Plan     string
	Password string // 管理员初始密码
}

// CreateTenantResponse 创建租户响应
type CreateTenantResponse struct {
	Tenant *store.Tenant
	Admin  *store.User
	APIKey *store.APIKey
}

// CreateTenant 创建新租户（含默认管理员和 API Key）
func (s *Service) CreateTenant(ctx context.Context, req *CreateTenantRequest) (*CreateTenantResponse, error) {
	// 使用数据库事务确保所有操作的原子性
	var result *CreateTenantResponse
	err := s.repo.DB().WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// 检查租户名是否已存在
		var existing store.Tenant
		if err := tx.Where("name = ?", req.Name).First(&existing).Error; err != nil {
			if !errors.Is(err, gorm.ErrRecordNotFound) {
				return fmt.Errorf("check tenant existence: %w", err)
			}
		} else {
			return ErrTenantExists
		}

		// 创建租户
		plan := req.Plan
		if plan == "" {
			plan = "free"
		}
		tenant := &store.Tenant{
			Name:   req.Name,
			Email:  req.Email,
			Phone:  req.Phone,
			Status: store.TenantStatusActive,
			Plan:   plan,
		}
		if err := tx.Create(tenant).Error; err != nil {
			return fmt.Errorf("create tenant: %w", err)
		}

		// 创建默认管理员账号
		hashedPwd, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
		if err != nil {
			return fmt.Errorf("hash password: %w", err)
		}
		admin := &store.User{
			TenantID: tenant.ID,
			Username: req.Name + "_admin",
			Password: string(hashedPwd),
			Role:     store.UserRoleAdmin,
			Status:   "active",
		}
		if err := tx.Create(admin).Error; err != nil {
			return fmt.Errorf("create admin user: %w", err)
		}

		// 生成默认 API Key
		apiKey := &store.APIKey{
			TenantID: tenant.ID,
			UserID:   admin.ID,
			Key:      generateAPIKey(),
			Name:     "Default API Key",
			Status:   "active",
		}
		if err := tx.Create(apiKey).Error; err != nil {
			return fmt.Errorf("create api key: %w", err)
		}

		// 初始化默认配额
		quota := &store.Quota{
			TenantID:      tenant.ID,
			MonthlyLimit:  getDefaultMonthlyLimit(plan),
			DailyLimit:    getDefaultDailyLimit(plan),
			RateLimit:     getDefaultRateLimit(plan),
			OveragePolicy: "throttle",
		}
		if err := tx.Create(quota).Error; err != nil {
			return fmt.Errorf("create quota: %w", err)
		}

		result = &CreateTenantResponse{
			Tenant: tenant,
			Admin:  admin,
			APIKey: apiKey,
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

// EnsureTenantResources 确保租户拥有完整的资源（API Key 和 Quota）
// 如果租户已存在但缺少 API Key 或 Quota，会自动补充创建缺失资源
func (s *Service) EnsureTenantResources(ctx context.Context, tenantName, plan, password string) (*CreateTenantResponse, error) {
	// 查找现有租户
	existing, err := s.repo.GetTenantByName(ctx, tenantName)
	if err != nil {
		return nil, fmt.Errorf("check tenant existence: %w", err)
	}
	if existing == nil {
		return nil, ErrTenantNotFound
	}

	// 使用数据库事务确保恢复操作的原子性
	var result *CreateTenantResponse
	err = s.repo.DB().WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// 检查租户是否已有 API Key
		var existingAPIKey store.APIKey
		hasAPIKey := true
		if err := tx.Where("tenant_id = ? AND status = ?", existing.ID, "active").First(&existingAPIKey).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				hasAPIKey = false
			} else {
				return fmt.Errorf("check api key existence: %w", err)
			}
		}

		var apiKey *store.APIKey
		if !hasAPIKey {
			// 查找租户的管理员账号
			var admin store.User
			if err := tx.Where("tenant_id = ? AND role = ?", existing.ID, store.UserRoleAdmin).First(&admin).Error; err != nil {
				if errors.Is(err, gorm.ErrRecordNotFound) {
					// 管理员也不存在，需要创建
					hashedPwd, hashErr := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
					if hashErr != nil {
						return fmt.Errorf("hash password: %w", hashErr)
					}
					admin = store.User{
						TenantID: existing.ID,
						Username: tenantName + "_admin",
						Password: string(hashedPwd),
						Role:     store.UserRoleAdmin,
						Status:   "active",
					}
					if err := tx.Create(&admin).Error; err != nil {
						return fmt.Errorf("create admin user: %w", err)
					}
				} else {
					return fmt.Errorf("check admin existence: %w", err)
				}
			}

			// 创建缺失的 API Key
			newAPIKey := &store.APIKey{
				TenantID: existing.ID,
				UserID:   admin.ID,
				Key:      generateAPIKey(),
				Name:     "Default API Key",
				Status:   "active",
			}
			if err := tx.Create(newAPIKey).Error; err != nil {
				return fmt.Errorf("create api key: %w", err)
			}
			apiKey = newAPIKey
		} else {
			apiKey = &existingAPIKey
		}

		// 检查租户是否已有 Quota
		var existingQuota store.Quota
		hasQuota := true
		if err := tx.Where("tenant_id = ?", existing.ID).First(&existingQuota).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				hasQuota = false
			} else {
				return fmt.Errorf("check quota existence: %w", err)
			}
		}

		var quota *store.Quota
		if !hasQuota {
			// 创建缺失的 Quota
			plan := existing.Plan
			if plan == "" {
				plan = "free"
			}
			newQuota := &store.Quota{
				TenantID:      existing.ID,
				MonthlyLimit:  getDefaultMonthlyLimit(plan),
				DailyLimit:    getDefaultDailyLimit(plan),
				RateLimit:     getDefaultRateLimit(plan),
				OveragePolicy: "throttle",
			}
			if err := tx.Create(newQuota).Error; err != nil {
				return fmt.Errorf("create quota: %w", err)
			}
			quota = newQuota
		} else {
			quota = &existingQuota
		}

		result = &CreateTenantResponse{
			Tenant: existing,
			Admin:  nil, // Admin 信息在此恢复场景中非必需
			APIKey: apiKey,
		}
		_ = quota // Quota 信息在此恢复场景中非必需
		return nil
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

// GetTenant 获取租户信息
func (s *Service) GetTenant(ctx context.Context, id uint) (*store.Tenant, error) {
	tenant, err := s.repo.GetTenantByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if tenant == nil {
		return nil, ErrTenantNotFound
	}
	return tenant, nil
}

// DisableTenant 禁用租户
func (s *Service) DisableTenant(ctx context.Context, id uint) error {
	tenant, err := s.repo.GetTenantByID(ctx, id)
	if err != nil {
		return err
	}
	if tenant == nil {
		return ErrTenantNotFound
	}
	tenant.Status = store.TenantStatusDisabled
	return s.repo.UpdateTenant(ctx, tenant)
}

// EnableTenant 启用租户
func (s *Service) EnableTenant(ctx context.Context, id uint) error {
	tenant, err := s.repo.GetTenantByID(ctx, id)
	if err != nil {
		return err
	}
	if tenant == nil {
		return ErrTenantNotFound
	}
	tenant.Status = store.TenantStatusActive
	return s.repo.UpdateTenant(ctx, tenant)
}

// CreateUser 创建子账号
func (s *Service) CreateUser(
	ctx context.Context, tenantID uint, username, password string, role store.UserRole,
) (*store.User, error) {
	// 检查用户名是否已存在
	existing, err := s.repo.GetUserByUsername(ctx, username)
	if err != nil {
		return nil, err
	}
	if existing != nil {
		return nil, ErrUserExists
	}

	hashedPwd, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, fmt.Errorf("hash password: %w", err)
	}

	user := &store.User{
		TenantID: tenantID,
		Username: username,
		Password: string(hashedPwd),
		Role:     role,
		Status:   "active",
	}
	if err := s.repo.CreateUser(ctx, user); err != nil {
		return nil, err
	}
	return user, nil
}

// CreateAPIKeyRequest 创建 API Key 请求
type CreateAPIKeyRequest struct {
	TenantID    uint
	UserID      uint
	Name        string
	Permissions string
	IPWhitelist string
	ExpiresAt   *time.Time
}

// CreateAPIKey 创建新的 API Key
func (s *Service) CreateAPIKey(ctx context.Context, req *CreateAPIKeyRequest) (*store.APIKey, error) {
	apiKey := &store.APIKey{
		TenantID:    req.TenantID,
		UserID:      req.UserID,
		Key:         generateAPIKey(),
		Name:        req.Name,
		Permissions: req.Permissions,
		IPWhitelist: req.IPWhitelist,
		Status:      "active",
		ExpiresAt:   req.ExpiresAt,
	}
	if err := s.repo.CreateAPIKey(ctx, apiKey); err != nil {
		return nil, err
	}
	return apiKey, nil
}

// RevokeAPIKey 吊销 API Key
func (s *Service) RevokeAPIKey(ctx context.Context, tenantID, keyID uint) error {
	keys, _, err := s.repo.ListAPIKeysByTenant(ctx, tenantID, 0, 1000)
	if err != nil {
		return err
	}
	for _, k := range keys {
		if k.ID == keyID {
			k.Status = "revoked"
			return s.repo.UpdateAPIKey(ctx, k)
		}
	}
	return errors.New("api key not found")
}

// ValidatePassword 验证用户密码
func (s *Service) ValidatePassword(ctx context.Context, username, password string) (*store.User, error) {
	user, err := s.repo.GetUserByUsername(ctx, username)
	if err != nil {
		return nil, err
	}
	if user == nil {
		return nil, ErrUserNotFound
	}
	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(password)); err != nil {
		return nil, ErrInvalidPassword
	}
	return user, nil
}

// generateAPIKey 生成随机 API Key
func generateAPIKey() string {
	// 生成 28 字节随机数 → hex 编码为 56 字符 → 加上 "cis_" 前缀共 60 字符
	// 满足数据库 key 字段 varchar(64) 的长度限制
	bytes := make([]byte, 28)
	if _, err := rand.Read(bytes); err != nil {
		// fallback: 使用时间戳
		return fmt.Sprintf("cis_%d", time.Now().UnixNano())
	}
	return "cis_" + hex.EncodeToString(bytes)
}

// 套餐名称常量
const (
	planDeveloper  = "developer"
	planEnterprise = "enterprise"
)

// 根据套餐获取默认月调用上限
func getDefaultMonthlyLimit(plan string) uint64 {
	switch plan {
	case planDeveloper:
		return 50000
	case planEnterprise:
		return 0 // 无限制
	default: // free
		return 1000
	}
}

// 根据套餐获取默认日调用上限
func getDefaultDailyLimit(plan string) uint64 {
	switch plan {
	case planDeveloper:
		return 5000
	case planEnterprise:
		return 0 // 无限制
	default: // free
		return 100
	}
}

// 根据套餐获取默认 QPS 限制
func getDefaultRateLimit(plan string) int {
	switch plan {
	case planDeveloper:
		return 50
	case planEnterprise:
		return 500
	default: // free
		return 10
	}
}
