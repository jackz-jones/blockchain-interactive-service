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
	// 检查租户名是否已存在
	existing, err := s.repo.GetTenantByName(ctx, req.Name)
	if err != nil {
		return nil, fmt.Errorf("check tenant existence: %w", err)
	}
	if existing != nil {
		return nil, ErrTenantExists
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
	if err := s.repo.CreateTenant(ctx, tenant); err != nil {
		return nil, fmt.Errorf("create tenant: %w", err)
	}

	// 创建默认管理员账号
	hashedPwd, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		return nil, fmt.Errorf("hash password: %w", err)
	}
	admin := &store.User{
		TenantID: tenant.ID,
		Username: req.Name + "_admin",
		Password: string(hashedPwd),
		Role:     store.UserRoleAdmin,
		Status:   "active",
	}
	if err := s.repo.CreateUser(ctx, admin); err != nil {
		return nil, fmt.Errorf("create admin user: %w", err)
	}

	// 生成默认 API Key
	apiKey := &store.APIKey{
		TenantID: tenant.ID,
		UserID:   admin.ID,
		Key:      generateAPIKey(),
		Name:     "Default API Key",
		Status:   "active",
	}
	if err := s.repo.CreateAPIKey(ctx, apiKey); err != nil {
		return nil, fmt.Errorf("create api key: %w", err)
	}

	// 初始化默认配额
	quota := &store.Quota{
		TenantID:      tenant.ID,
		MonthlyLimit:  getDefaultMonthlyLimit(plan),
		DailyLimit:    getDefaultDailyLimit(plan),
		RateLimit:     getDefaultRateLimit(plan),
		OveragePolicy: "throttle",
	}
	if err := s.repo.CreateOrUpdateQuota(ctx, quota); err != nil {
		return nil, fmt.Errorf("create quota: %w", err)
	}

	return &CreateTenantResponse{
		Tenant: tenant,
		Admin:  admin,
		APIKey: apiKey,
	}, nil
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
func (s *Service) CreateUser(ctx context.Context, tenantID uint, username, password string, role store.UserRole) (*store.User, error) {
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
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		// fallback: 使用时间戳
		return fmt.Sprintf("cis_%d", time.Now().UnixNano())
	}
	return "cis_" + hex.EncodeToString(bytes)
}

// 根据套餐获取默认月调用上限
func getDefaultMonthlyLimit(plan string) uint64 {
	switch plan {
	case "developer":
		return 50000
	case "enterprise":
		return 0 // 无限制
	default: // free
		return 1000
	}
}

// 根据套餐获取默认日调用上限
func getDefaultDailyLimit(plan string) uint64 {
	switch plan {
	case "developer":
		return 5000
	case "enterprise":
		return 0 // 无限制
	default: // free
		return 100
	}
}

// 根据套餐获取默认 QPS 限制
func getDefaultRateLimit(plan string) int {
	switch plan {
	case "developer":
		return 50
	case "enterprise":
		return 500
	default: // free
		return 10
	}
}
