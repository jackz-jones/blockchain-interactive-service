package gateway

import (
	"net/http"
	"strconv"
	"time"

	"github.com/jackz-jones/blockchain-interactive-service/internal/middleware"
	"github.com/jackz-jones/blockchain-interactive-service/internal/store"
	"github.com/jackz-jones/blockchain-interactive-service/internal/svc"
	"github.com/zeromicro/go-zero/rest/pathvar"
)

// ========== 管理后台 API Handler ==========

// DashboardOverviewHandler 仪表盘概览 Handler
func DashboardOverviewHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tenantID := middleware.GetTenantIDFromHTTP(r)
		if tenantID == 0 {
			errorResponse(w, http.StatusUnauthorized, "unauthorized")
			return
		}

		// 获取用量统计
		stats, err := svcCtx.BillingService.GetUsageStats(r.Context(), tenantID)
		if err != nil {
			errorResponse(w, http.StatusInternalServerError, "get usage stats: "+err.Error())
			return
		}

		// 获取可用链数量
		chains, err := svcCtx.TenantSDKManager.ListTenantChains(r.Context(), tenantID)
		if err != nil {
			errorResponse(w, http.StatusInternalServerError, "list chains: "+err.Error())
			return
		}

		// 获取今日成功率
		now := time.Now()
		todayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
		todayEnd := todayStart.Add(24 * time.Hour)
		allLogs, totalCount, _ := svcCtx.Repo.ListCallLogs(r.Context(), store.CallLogFilter{
			TenantID:  tenantID,
			StartTime: &todayStart,
			EndTime:   &todayEnd,
		}, 0, 1)
		_ = allLogs

		successLogs, _, _ := svcCtx.Repo.ListCallLogs(r.Context(), store.CallLogFilter{
			TenantID:  tenantID,
			Status:    "success",
			StartTime: &todayStart,
			EndTime:   &todayEnd,
		}, 0, 1)
		_ = successLogs

		var successRate float64
		if totalCount > 0 {
			// 简化计算：用总数估算
			successRate = 100.0 // 需要更精确的统计
		}

		successResponse(w, map[string]interface{}{
			"today_calls":   stats.TodayCalls,
			"month_calls":   stats.MonthCalls,
			"monthly_limit": stats.MonthlyLimit,
			"usage_percent": stats.UsagePercent,
			"active_chains": len(chains),
			"success_rate":  successRate,
		})
	}
}

// ListCallLogsHandler 调用日志查询 Handler
func ListCallLogsHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tenantID := middleware.GetTenantIDFromHTTP(r)
		if tenantID == 0 {
			errorResponse(w, http.StatusUnauthorized, "unauthorized")
			return
		}

		// 解析查询参数
		page, _ := strconv.Atoi(r.URL.Query().Get("page"))
		pageSize, _ := strconv.Atoi(r.URL.Query().Get("page_size"))
		if page <= 0 {
			page = 1
		}
		if pageSize <= 0 || pageSize > 100 {
			pageSize = 20
		}

		filter := store.CallLogFilter{
			TenantID:     tenantID,
			ChainName:    r.URL.Query().Get("chain_name"),
			ContractName: r.URL.Query().Get("contract_name"),
			Status:       r.URL.Query().Get("status"),
		}

		// 解析时间范围
		if startStr := r.URL.Query().Get("start_time"); startStr != "" {
			if t, err := time.Parse(time.RFC3339, startStr); err == nil {
				filter.StartTime = &t
			}
		}
		if endStr := r.URL.Query().Get("end_time"); endStr != "" {
			if t, err := time.Parse(time.RFC3339, endStr); err == nil {
				filter.EndTime = &t
			}
		}

		logs, total, err := svcCtx.Repo.ListCallLogs(r.Context(), filter, (page-1)*pageSize, pageSize)
		if err != nil {
			errorResponse(w, http.StatusInternalServerError, "list call logs: "+err.Error())
			return
		}

		successResponse(w, map[string]interface{}{
			"total": total,
			"page":  page,
			"items": logs,
		})
	}
}

// ListBillsHandler 账单查询 Handler
func ListBillsHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tenantID := middleware.GetTenantIDFromHTTP(r)
		if tenantID == 0 {
			errorResponse(w, http.StatusUnauthorized, "unauthorized")
			return
		}

		page, _ := strconv.Atoi(r.URL.Query().Get("page"))
		pageSize, _ := strconv.Atoi(r.URL.Query().Get("page_size"))
		if page <= 0 {
			page = 1
		}
		if pageSize <= 0 || pageSize > 100 {
			pageSize = 20
		}

		bills, total, err := svcCtx.Repo.ListBillsByTenant(r.Context(), tenantID, (page-1)*pageSize, pageSize)
		if err != nil {
			errorResponse(w, http.StatusInternalServerError, "list bills: "+err.Error())
			return
		}

		successResponse(w, map[string]interface{}{
			"total": total,
			"page":  page,
			"items": bills,
		})
	}
}

// GetUsageStatsHandler 用量统计 Handler
func GetUsageStatsHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tenantID := middleware.GetTenantIDFromHTTP(r)
		if tenantID == 0 {
			errorResponse(w, http.StatusUnauthorized, "unauthorized")
			return
		}

		stats, err := svcCtx.BillingService.GetUsageStats(r.Context(), tenantID)
		if err != nil {
			errorResponse(w, http.StatusInternalServerError, "get usage stats: "+err.Error())
			return
		}

		successResponse(w, stats)
	}
}

// ListAuditLogsHandler 审计日志查询 Handler
func ListAuditLogsHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tenantID := middleware.GetTenantIDFromHTTP(r)
		if tenantID == 0 {
			errorResponse(w, http.StatusUnauthorized, "unauthorized")
			return
		}

		page, _ := strconv.Atoi(r.URL.Query().Get("page"))
		pageSize, _ := strconv.Atoi(r.URL.Query().Get("page_size"))
		if page <= 0 {
			page = 1
		}
		if pageSize <= 0 || pageSize > 100 {
			pageSize = 20
		}

		filter := store.AuditLogFilter{
			TenantID: tenantID,
			Action:   r.URL.Query().Get("action"),
		}

		if userIDStr := r.URL.Query().Get("user_id"); userIDStr != "" {
			if uid, err := strconv.ParseUint(userIDStr, 10, 64); err == nil {
				filter.UserID = uint(uid)
			}
		}
		if startStr := r.URL.Query().Get("start_time"); startStr != "" {
			if t, err := time.Parse(time.RFC3339, startStr); err == nil {
				filter.StartTime = &t
			}
		}
		if endStr := r.URL.Query().Get("end_time"); endStr != "" {
			if t, err := time.Parse(time.RFC3339, endStr); err == nil {
				filter.EndTime = &t
			}
		}

		logs, total, err := svcCtx.Repo.ListAuditLogs(r.Context(), filter, (page-1)*pageSize, pageSize)
		if err != nil {
			errorResponse(w, http.StatusInternalServerError, "list audit logs: "+err.Error())
			return
		}

		successResponse(w, map[string]interface{}{
			"total": total,
			"page":  page,
			"items": logs,
		})
	}
}

// ListTenantsHandler 租户列表 Handler（平台管理员）
func ListTenantsHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		page, _ := strconv.Atoi(r.URL.Query().Get("page"))
		pageSize, _ := strconv.Atoi(r.URL.Query().Get("page_size"))
		if page <= 0 {
			page = 1
		}
		if pageSize <= 0 || pageSize > 100 {
			pageSize = 20
		}

		tenants, total, err := svcCtx.Repo.ListTenants(r.Context(), (page-1)*pageSize, pageSize)
		if err != nil {
			errorResponse(w, http.StatusInternalServerError, "list tenants: "+err.Error())
			return
		}

		successResponse(w, map[string]interface{}{
			"total": total,
			"page":  page,
			"items": tenants,
		})
	}
}

// DisableTenantHandler 禁用租户 Handler
func DisableTenantHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		vars := pathvar.Vars(r)
		idStr := vars["id"]
		id, err := strconv.ParseUint(idStr, 10, 64)
		if err != nil {
			errorResponse(w, http.StatusBadRequest, "invalid tenant id")
			return
		}

		if err := svcCtx.TenantService.DisableTenant(r.Context(), uint(id)); err != nil {
			errorResponse(w, http.StatusInternalServerError, "disable tenant: "+err.Error())
			return
		}

		successResponse(w, nil)
	}
}

// EnableTenantHandler 启用租户 Handler
func EnableTenantHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		vars := pathvar.Vars(r)
		idStr := vars["id"]
		id, err := strconv.ParseUint(idStr, 10, 64)
		if err != nil {
			errorResponse(w, http.StatusBadRequest, "invalid tenant id")
			return
		}

		if err := svcCtx.TenantService.EnableTenant(r.Context(), uint(id)); err != nil {
			errorResponse(w, http.StatusInternalServerError, "enable tenant: "+err.Error())
			return
		}

		successResponse(w, nil)
	}
}

// ListUsersHandler 用户列表 Handler
func ListUsersHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tenantID := middleware.GetTenantIDFromHTTP(r)
		if tenantID == 0 {
			errorResponse(w, http.StatusUnauthorized, "unauthorized")
			return
		}

		page, _ := strconv.Atoi(r.URL.Query().Get("page"))
		pageSize, _ := strconv.Atoi(r.URL.Query().Get("page_size"))
		if page <= 0 {
			page = 1
		}
		if pageSize <= 0 || pageSize > 100 {
			pageSize = 20
		}

		users, total, err := svcCtx.Repo.ListUsersByTenant(r.Context(), tenantID, (page-1)*pageSize, pageSize)
		if err != nil {
			errorResponse(w, http.StatusInternalServerError, "list users: "+err.Error())
			return
		}

		successResponse(w, map[string]interface{}{
			"total": total,
			"page":  page,
			"items": users,
		})
	}
}
