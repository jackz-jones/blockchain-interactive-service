package billing

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/jackz-jones/blockchain-interactive-service/internal/store"
	"github.com/zeromicro/go-zero/core/logx"
)

// Service 计费与配额服务
type Service struct {
	repo store.Repository

	// dailyCounters 日调用计数器缓存: tenantID -> count（内存缓存，定期同步到 DB）
	dailyCounters sync.Map

	// monthlyCounters 月调用计数器缓存
	monthlyCounters sync.Map

	logger logx.Logger
}

// NewService 创建计费服务
func NewService(repo store.Repository, logger logx.Logger) *Service {
	s := &Service{
		repo:   repo,
		logger: logger,
	}
	return s
}

// CheckQuota 检查租户配额是否允许本次调用
// 返回值：allowed（是否允许）、warning（是否接近上限）、err
func (s *Service) CheckQuota(ctx context.Context, tenantID uint) (allowed bool, warning bool, err error) {
	quota, err := s.repo.GetQuotaByTenant(ctx, tenantID)
	if err != nil {
		return false, false, fmt.Errorf("get quota: %w", err)
	}
	if quota == nil {
		// 没有配额记录，默认允许（兼容旧数据）
		return true, false, nil
	}

	// 企业版无限制
	if quota.MonthlyLimit == 0 && quota.DailyLimit == 0 {
		return true, false, nil
	}

	// 检查日配额
	if quota.DailyLimit > 0 {
		dailyCount, err := s.getDailyCount(ctx, tenantID)
		if err != nil {
			return false, false, err
		}
		if uint64(dailyCount) >= quota.DailyLimit {
			if quota.OveragePolicy == "block" {
				return false, false, nil
			}
			// throttle 模式下仍然返回不允许，但由上层决定是限流还是拒绝
			return false, false, nil
		}
	}

	// 检查月配额
	if quota.MonthlyLimit > 0 {
		monthlyUsed := quota.MonthlyUsed
		if monthlyUsed >= quota.MonthlyLimit {
			return false, false, nil
		}

		// 检查是否达到 80% 预警线
		warningThreshold := quota.MonthlyLimit * 80 / 100
		if monthlyUsed >= warningThreshold {
			return true, true, nil
		}
	}

	return true, false, nil
}

// RecordUsage 记录一次调用用量
func (s *Service) RecordUsage(ctx context.Context, tenantID uint) error {
	// 增加月用量计数
	if err := s.repo.IncrementMonthlyUsed(ctx, tenantID, 1); err != nil {
		s.logger.Errorf("increment monthly used for tenant %d: %v", tenantID, err)
		return err
	}

	// 更新内存计数器
	s.incrementDailyCount(tenantID)

	return nil
}

// GenerateMonthlyBills 生成月度账单（定时任务调用）
func (s *Service) GenerateMonthlyBills(ctx context.Context) error {
	now := time.Now()
	// 生成上个月的账单
	year := now.Year()
	month := now.Month() - 1
	if month == 0 {
		month = 12
		year--
	}

	periodStart := time.Date(year, month, 1, 0, 0, 0, 0, time.Local)
	periodEnd := periodStart.AddDate(0, 1, 0)

	// 获取所有租户
	tenants, _, err := s.repo.ListTenants(ctx, 0, 10000)
	if err != nil {
		return fmt.Errorf("list tenants: %w", err)
	}

	for _, t := range tenants {
		if err := s.generateBillForTenant(ctx, t.ID, t.Plan, periodStart, periodEnd, year, month); err != nil {
			s.logger.Errorf("generate bill for tenant %d: %v", t.ID, err)
			continue
		}
	}

	s.logger.Infof("monthly bills generated for %d-%02d, %d tenants processed", year, month, len(tenants))
	return nil
}

// ResetMonthlyCounters 重置月度计数器（每月初调用）
func (s *Service) ResetMonthlyCounters(ctx context.Context) error {
	tenants, _, err := s.repo.ListTenants(ctx, 0, 10000)
	if err != nil {
		return err
	}

	for _, t := range tenants {
		quota, err := s.repo.GetQuotaByTenant(ctx, t.ID)
		if err != nil || quota == nil {
			continue
		}
		quota.MonthlyUsed = 0
		_ = s.repo.CreateOrUpdateQuota(ctx, quota)
	}

	s.logger.Info("monthly counters reset")
	return nil
}

// GetUsageStats 获取租户用量统计
func (s *Service) GetUsageStats(ctx context.Context, tenantID uint) (*UsageStats, error) {
	now := time.Now()

	// 今日调用量
	todayCount, err := s.repo.CountCallsByTenantToday(ctx, tenantID)
	if err != nil {
		return nil, err
	}

	// 本月调用量
	monthCount, err := s.repo.CountCallsByTenantMonth(ctx, tenantID, now.Year(), now.Month())
	if err != nil {
		return nil, err
	}

	// 配额信息
	quota, err := s.repo.GetQuotaByTenant(ctx, tenantID)
	if err != nil {
		return nil, err
	}

	stats := &UsageStats{
		TodayCalls:   todayCount,
		MonthCalls:   monthCount,
		MonthlyLimit: 0,
		DailyLimit:   0,
		UsagePercent: 0,
	}

	if quota != nil {
		stats.MonthlyLimit = int64(quota.MonthlyLimit)
		stats.DailyLimit = int64(quota.DailyLimit)
		if quota.MonthlyLimit > 0 {
			stats.UsagePercent = float64(monthCount) / float64(quota.MonthlyLimit) * 100
		}
	}

	return stats, nil
}

// UsageStats 用量统计
type UsageStats struct {
	TodayCalls   int64   `json:"today_calls"`
	MonthCalls   int64   `json:"month_calls"`
	MonthlyLimit int64   `json:"monthly_limit"`
	DailyLimit   int64   `json:"daily_limit"`
	UsagePercent float64 `json:"usage_percent"`
}

// ========== 内部方法 ==========

// getDailyCount 获取租户今日调用次数
func (s *Service) getDailyCount(ctx context.Context, tenantID uint) (int64, error) {
	// 优先从内存缓存获取
	if count, ok := s.dailyCounters.Load(tenantID); ok {
		return count.(int64), nil
	}

	// 从数据库查询
	count, err := s.repo.CountCallsByTenantToday(ctx, tenantID)
	if err != nil {
		return 0, err
	}

	s.dailyCounters.Store(tenantID, count)
	return count, nil
}

// incrementDailyCount 增加日计数器
func (s *Service) incrementDailyCount(tenantID uint) {
	if count, ok := s.dailyCounters.Load(tenantID); ok {
		s.dailyCounters.Store(tenantID, count.(int64)+1)
	} else {
		s.dailyCounters.Store(tenantID, int64(1))
	}
}

// generateBillForTenant 为单个租户生成账单
func (s *Service) generateBillForTenant(ctx context.Context, tenantID uint, plan string,
	periodStart, periodEnd time.Time, year int, month time.Month) error {

	// 统计该月调用量
	totalCalls, err := s.repo.CountCallsByTenantMonth(ctx, tenantID, year, month)
	if err != nil {
		return err
	}

	if totalCalls == 0 {
		return nil // 无调用不生成账单
	}

	// 计算费用
	amount := calculateAmount(plan, uint64(totalCalls))

	bill := &store.Bill{
		TenantID:    tenantID,
		PeriodStart: periodStart,
		PeriodEnd:   periodEnd,
		TotalCalls:  uint64(totalCalls),
		Amount:      amount,
		Currency:    "CNY",
		Status:      "unpaid",
	}

	return s.repo.CreateBill(ctx, bill)
}

// calculateAmount 根据套餐和调用量计算费用
func calculateAmount(plan string, totalCalls uint64) float64 {
	switch plan {
	case "free":
		// 免费层超出 1000 次后按 0.01 元/次计费
		if totalCalls <= 1000 {
			return 0
		}
		return float64(totalCalls-1000) * 0.01

	case "developer":
		// 开发者版：月费 99 元含 50000 次，超出按 0.005 元/次
		baseFee := 99.0
		if totalCalls <= 50000 {
			return baseFee
		}
		return baseFee + float64(totalCalls-50000)*0.005

	case "enterprise":
		// 企业版：月费 999 元，无限调用
		return 999.0

	default:
		return float64(totalCalls) * 0.01
	}
}
