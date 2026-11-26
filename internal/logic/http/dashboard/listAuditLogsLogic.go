package dashboard

import (
	"context"
	"strconv"
	"time"

	"github.com/jackz-jones/blockchain-interactive-service/internal/middleware"
	"github.com/jackz-jones/blockchain-interactive-service/internal/store"
	"github.com/jackz-jones/blockchain-interactive-service/internal/svc"
	"github.com/jackz-jones/blockchain-interactive-service/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

// 提取的重复字符串常量（goconst）
const (
	chainConfigTest   = "chain_config_test"
	contractConfigStr = "contract_config"
	chainConfigs      = "chain-configs"
	chainConfigStr    = "chain_config"
	contractsStr      = "contracts"
	apiKeyStr         = "api_key"
	tenantsStr        = "tenants"
	tenantStr         = "tenant"
	usersStr          = "users"
	userStr           = "user"
	apiKeysStr        = "api-keys"
	// 资源类型中文标签（resource_label 展示用）
	labelChainConfig       = "链配置"
	labelContractConfig    = "合约配置"
	labelAPIKey            = "API 密钥"
	labelEventSubscription = "事件订阅"
	labelBill              = "账单"
	labelTenant            = "租户"
	labelUser              = "用户"
)

type ListAuditLogsLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewListAuditLogsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListAuditLogsLogic {
	return &ListAuditLogsLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *ListAuditLogsLogic) ListAuditLogs(req *types.ListAuditLogsRequest) (resp *types.CommonResponse, err error) {
	tenantID := middleware.GetTenantIDFromContext(l.ctx)
	if tenantID == 0 {
		return &types.CommonResponse{Code: 401, Message: unauthorizedStr}, nil
	}

	page := req.Page
	pageSize := req.PageSize
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 || pageSize > 100 {
		pageSize = 20
	}

	filter := store.AuditLogFilter{
		TenantID: tenantID,
		Action:   req.Action,
		UserID:   req.UserId,
	}

	if req.StartTime != "" {
		if t, err := time.Parse(time.RFC3339, req.StartTime); err == nil {
			filter.StartTime = &t
		}
	}
	if req.EndTime != "" {
		if t, err := time.Parse(time.RFC3339, req.EndTime); err == nil {
			filter.EndTime = &t
		}
	}

	logs, total, err := l.svcCtx.Repo.ListAuditLogs(l.ctx, filter, (page-1)*pageSize, pageSize)
	if err != nil {
		return &types.CommonResponse{Code: 500, Message: "list audit logs: " + err.Error()}, nil
	}

	// 填充虚拟字段
	for _, log := range logs {
		log.CreatedAt = log.Model.CreatedAt
		// Operator 已通过 Joins 从 users 表填充（见 repository.go ListAuditLogs 方法）
		// ResourceID 已通过 AuditLog.ResourceID 字段保存

		// 关联查询资源名称，将数据库ID替换为有意义的名称
		if log.Resource != "" && log.ResourceID != "" {
			log.ResourceName = l.resolveResourceName(log.Resource, log.ResourceID)
		}

		// 填充资源类型中文名
		log.ResourceLabel = resolveResourceLabel(log.Resource)
	}

	return &types.CommonResponse{
		Code:    0,
		Message: successStr,
		Data: map[string]interface{}{
			totalStr: total,
			pageStr:  page,
			itemsStr: logs,
		},
	}, nil
}

// resolveResourceLabel 将资源类型标识映射为用户友好的中文名
func resolveResourceLabel(resourceType string) string {
	labels := map[string]string{
		chainConfigs:      labelChainConfig,
		chainConfigStr:    labelChainConfig,
		chainConfigTest:   labelChainConfig,
		contractConfigStr: labelContractConfig,
		contractsStr:      labelContractConfig,
		"contract_call":   "合约调用",
		apiKeysStr:        labelAPIKey,
		apiKeyStr:         labelAPIKey,
		tenantsStr:        labelTenant,
		tenantStr:         labelTenant,
		usersStr:          labelUser,
		userStr:           labelUser,
		"events":          labelEventSubscription,
		"event":           labelEventSubscription,
		"bills":           labelBill,
		"bill":            labelBill,
	}
	if label, ok := labels[resourceType]; ok {
		return label
	}
	return resourceType
}

// resolveResourceName 根据资源类型和ID查询有意义的名称
func (l *ListAuditLogsLogic) resolveResourceName(resourceType, resourceID string) string {
	if resourceID == "" {
		return ""
	}
	if id, ok := parseResourceID(resourceID); ok {
		if name, ok := l.lookupResourceName(resourceType, id); ok {
			return name
		}
	}
	return resourceID
}

// parseResourceID 解析资源 ID；失败时回退原值
func parseResourceID(resourceID string) (uint, bool) {
	id, err := strconv.ParseUint(resourceID, 10, 64)
	if err != nil {
		return 0, false
	}
	return uint(id), true
}

// lookupResourceName 按资源类型查询展示名称
func (l *ListAuditLogsLogic) lookupResourceName(resourceType string, id uint) (string, bool) {
	switch resourceType {
	case chainConfigs, chainConfigStr, chainConfigTest:
		chainConfig, err := l.svcCtx.Repo.GetChainConfigByID(l.ctx, id)
		if err != nil || chainConfig == nil {
			return "", false
		}
		return chainConfig.ChainName, true
	case contractConfigStr, contractsStr:
		contractConfig, err := l.svcCtx.Repo.GetContractConfig(l.ctx, id)
		if err != nil || contractConfig == nil {
			return "", false
		}
		return contractConfig.ContractName, true
	case apiKeysStr, apiKeyStr:
		// 通过 DB 直接查询 API Key 的名称
		var apiKey store.APIKey
		if err := l.svcCtx.Repo.DB().WithContext(l.ctx).Select("name").First(&apiKey, id).Error; err != nil {
			return "", false
		}
		return apiKey.Name, apiKey.Name != ""
	case tenantsStr, tenantStr:
		tenant, err := l.svcCtx.Repo.GetTenantByID(l.ctx, id)
		if err != nil || tenant == nil {
			return "", false
		}
		return tenant.Name, true
	case usersStr, userStr:
		user, err := l.svcCtx.Repo.GetUserByID(l.ctx, id)
		if err != nil || user == nil {
			return "", false
		}
		return user.Username, true
	default:
		return "", false
	}
}
