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
		return &types.CommonResponse{Code: 401, Message: "unauthorized"}, nil
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
		Message: "success",
		Data: map[string]interface{}{
			"total": total,
			"page":  page,
			"items": logs,
		},
	}, nil
}

// resolveResourceLabel 将资源类型标识映射为用户友好的中文名
func resolveResourceLabel(resourceType string) string {
	labels := map[string]string{
		"chain-configs":     "链配置",
		"chain_config":      "链配置",
		"chain_config_test": "链配置",
		"contract_config":   "合约配置",
		"contracts":         "合约配置",
		"contract_call":     "合约调用",
		"api-keys":          "API 密钥",
		"api_key":           "API 密钥",
		"tenants":           "租户",
		"tenant":            "租户",
		"users":             "用户",
		"user":              "用户",
		"events":            "事件订阅",
		"event":             "事件订阅",
		"bills":             "账单",
		"bill":              "账单",
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

	switch resourceType {
	case "chain-configs", "chain_config", "chain_config_test":
		id, err := strconv.ParseUint(resourceID, 10, 64)
		if err != nil {
			return resourceID
		}
		chainConfig, err := l.svcCtx.Repo.GetChainConfigByID(l.ctx, uint(id))
		if err != nil || chainConfig == nil {
			return resourceID
		}
		return chainConfig.ChainName
	case "contract_config", "contracts":
		id, err := strconv.ParseUint(resourceID, 10, 64)
		if err != nil {
			return resourceID
		}
		contractConfig, err := l.svcCtx.Repo.GetContractConfig(l.ctx, uint(id))
		if err != nil || contractConfig == nil {
			return resourceID
		}
		return contractConfig.ContractName
	case "api-keys", "api_key":
		id, err := strconv.ParseUint(resourceID, 10, 64)
		if err != nil {
			return resourceID
		}
		// 通过 DB 直接查询 API Key 的名称
		var apiKey store.APIKey
		if err := l.svcCtx.Repo.DB().WithContext(l.ctx).Select("name").First(&apiKey, id).Error; err != nil {
			return resourceID
		}
		if apiKey.Name != "" {
			return apiKey.Name
		}
		return resourceID
	case "tenants", "tenant":
		id, err := strconv.ParseUint(resourceID, 10, 64)
		if err != nil {
			return resourceID
		}
		tenant, err := l.svcCtx.Repo.GetTenantByID(l.ctx, uint(id))
		if err != nil || tenant == nil {
			return resourceID
		}
		return tenant.Name
	case "users", "user":
		id, err := strconv.ParseUint(resourceID, 10, 64)
		if err != nil {
			return resourceID
		}
		user, err := l.svcCtx.Repo.GetUserByID(l.ctx, uint(id))
		if err != nil || user == nil {
			return resourceID
		}
		return user.Username
	default:
		return resourceID
	}
}
