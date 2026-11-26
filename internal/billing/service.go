package billing

import (
	"context"
	"fmt"
	"math"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/jackz-jones/blockchain-interactive-service/internal/store"
	"github.com/zeromicro/go-zero/core/logx"
)

// QuotaDecision CheckQuota 结果，区分 block / throttle 语义
type QuotaDecision struct {
	Allowed   bool // 是否允许通过
	Throttled bool // 命中 throttle 策略（配合 429 语义），仅在 Allowed=false 时有意义
	Warning   bool // 用量已接近上限
}

// Service 计费与配额服务
type Service struct {
	repo store.Repository

	// dailyCounters 日调用计数器缓存: "tenantID:YYYY-MM-DD" -> *atomic.Int64
	// key 中带日期，天然按日隔离，跨天不会串扰。
	dailyCounters sync.Map

	logger logx.Logger

	// loc 计费系统使用的时区，默认 Asia/Shanghai，可通过 SetLocation 自定义
	// 避免依赖 time.Local（容器中常为 UTC，会导致日/月账单周期偏移）
	loc *time.Location

	// 后台清理协程控制
	stopCh   chan struct{}
	stopOnce sync.Once
}

// NewService 创建计费服务
func NewService(repo store.Repository, logger logx.Logger) *Service {
	// 默认商业时区为 Asia/Shanghai；若加载失败（时区数据缺失）则回退到 UTC+8 固定偏移
	loc, err := time.LoadLocation("Asia/Shanghai")
	if err != nil || loc == nil {
		loc = time.FixedZone("CST", 8*3600)
	}
	s := &Service{
		repo:   repo,
		logger: logger,
		loc:    loc,
		stopCh: make(chan struct{}),
	}
	go s.startDailyCounterGC()
	return s
}

// SetLocation 自定义计费时区（应在服务启动时设置）
func (s *Service) SetLocation(loc *time.Location) {
	if loc == nil {
		return
	}
	s.loc = loc
}

// now 返回当前计费时区下的时间
func (s *Service) now() time.Time {
	return time.Now().In(s.loc)
}

// Stop 停止后台协程（用于测试或优雅关闭）
func (s *Service) Stop() {
	s.stopOnce.Do(func() {
		close(s.stopCh)
	})
}

// startDailyCounterGC 每小时清理一次昨日及更早的 daily counter key，防止内存无限增长
func (s *Service) startDailyCounterGC() {
	ticker := time.NewTicker(time.Hour)
	defer ticker.Stop()
	for {
		select {
		case <-s.stopCh:
			return
		case <-ticker.C:
			today := time.Now().Format("2006-01-02")
			suffix := ":" + today
			s.dailyCounters.Range(func(key, _ interface{}) bool {
				k, ok := key.(string)
				if !ok {
					return true
				}
				if !strings.HasSuffix(k, suffix) {
					s.dailyCounters.Delete(key)
				}
				return true
			})
		}
	}
}

// dailyKey 生成 tenantID+当前日期 的组合 key，天然按日隔离
func dailyKey(tenantID uint) string {
	return fmt.Sprintf("%d:%s", tenantID, time.Now().Format("2006-01-02"))
}

// CheckQuota 检查租户配额是否允许本次调用
// 返回 QuotaDecision 以区分 block / throttle 策略，方便上层选择 403 / 429 状态码
func (s *Service) CheckQuota(ctx context.Context, tenantID uint) (QuotaDecision, error) {
	quota, err := s.repo.GetQuotaByTenant(ctx, tenantID)
	if err != nil {
		return QuotaDecision{}, fmt.Errorf("get quota: %w", err)
	}
	if quota == nil {
		// 没有配额记录，默认允许（兼容旧数据）
		return QuotaDecision{Allowed: true}, nil
	}

	// 企业版无限制
	if quota.MonthlyLimit == 0 && quota.DailyLimit == 0 {
		return QuotaDecision{Allowed: true}, nil
	}

	// 检查日配额
	if quota.DailyLimit > 0 {
		dailyCount, err := s.getDailyCount(ctx, tenantID)
		if err != nil {
			return QuotaDecision{}, err
		}
		if uint64(dailyCount) >= quota.DailyLimit {
			throttled := quota.OveragePolicy == "throttle"
			return QuotaDecision{Allowed: false, Throttled: throttled}, nil
		}
	}

	// 检查月配额
	if quota.MonthlyLimit > 0 {
		monthlyUsed := quota.MonthlyUsed
		if monthlyUsed >= quota.MonthlyLimit {
			throttled := quota.OveragePolicy == "throttle"
			return QuotaDecision{Allowed: false, Throttled: throttled}, nil
		}

		// 检查是否达到 80% 预警线
		warningThreshold := quota.MonthlyLimit * 80 / 100
		if monthlyUsed >= warningThreshold {
			return QuotaDecision{Allowed: true, Warning: true}, nil
		}
	}

	return QuotaDecision{Allowed: true}, nil
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

// GenerateDailyBills 生成日账单（定时任务调用，每日24点执行）
// 所有日期边界均以 s.loc 为准，避免依赖 time.Local
func (s *Service) GenerateDailyBills(ctx context.Context) error {
	now := s.now()
	// 生成昨天的日账单
	yesterday := now.AddDate(0, 0, -1)
	year := yesterday.Year()
	month := yesterday.Month()
	day := yesterday.Day()

	periodStart := time.Date(year, month, day, 0, 0, 0, 0, s.loc)
	periodEnd := periodStart.AddDate(0, 0, 1)

	// 获取所有租户
	tenants, _, err := s.repo.ListTenants(ctx, 0, 10000)
	if err != nil {
		return fmt.Errorf("list tenants: %w", err)
	}

	generated := 0
	for _, t := range tenants {
		// 统计昨日 Invoke 调用量（仅计费写操作）
		totalCalls, err := s.repo.CountCallsByTenantDay(ctx, t.ID, year, month, day)
		if err != nil {
			s.logger.Errorf("count daily calls for tenant %d: %v", t.ID, err)
			continue
		}
		if totalCalls == 0 {
			continue // 无调用不生成日账单
		}
		if err := s.generateBillForTenant(ctx, t.ID, t.Plan, periodStart, periodEnd, "daily", totalCalls); err != nil {
			s.logger.Errorf("generate daily bill for tenant %d: %v", t.ID, err)
			continue
		}
		generated++
	}

	s.logger.Infof("daily bills generated for %d-%02d-%02d, %d/%d tenants processed", year, month, day, generated, len(tenants))
	return nil
}

// GenerateMonthlyBills 生成月度汇总账单（定时任务调用，每月1日执行）
func (s *Service) GenerateMonthlyBills(ctx context.Context) error {
	now := s.now()
	// 生成上个月的账单
	year := now.Year()
	month := now.Month() - 1
	if month == 0 {
		month = 12
		year--
	}

	periodStart := time.Date(year, month, 1, 0, 0, 0, 0, s.loc)
	periodEnd := periodStart.AddDate(0, 1, 0)

	// 获取所有租户
	tenants, _, err := s.repo.ListTenants(ctx, 0, 10000)
	if err != nil {
		return fmt.Errorf("list tenants: %w", err)
	}

	for _, t := range tenants {
		// 统计该月 Invoke 调用量（仅计费写操作）
		totalCalls, err := s.repo.CountCallsByTenantMonth(ctx, t.ID, year, month)
		if err != nil {
			s.logger.Errorf("count calls for tenant %d: %v", t.ID, err)
			continue
		}
		if err := s.generateBillForTenant(ctx, t.ID, t.Plan, periodStart, periodEnd, "monthly", totalCalls); err != nil {
			s.logger.Errorf("generate monthly bill for tenant %d: %v", t.ID, err)
			continue
		}
	}

	s.logger.Infof("monthly bills generated for %d-%02d, %d tenants processed", year, month, len(tenants))
	return nil
}

// ResetMonthlyCounters 重置月度计数器（每月初调用）
// 使用单条 UPDATE 批量重置，避免逐个 Save 导致的 N 次写入。
func (s *Service) ResetMonthlyCounters(ctx context.Context) error {
	n, err := s.repo.ResetAllMonthlyUsed(ctx)
	if err != nil {
		s.logger.Errorf("reset monthly counters failed: %v", err)
		return err
	}
	s.logger.Infof("monthly counters reset, affected rows=%d", n)
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
			stats.UsagePercent = math.Round(float64(monthCount)/float64(quota.MonthlyLimit)*100*10000) / 10000
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

// UsageStatsTrend 用量统计趋势数据
type UsageStatsTrend struct {
	Dates      []string `json:"dates"`
	Calls      []int64  `json:"calls"`
	Success    []int64  `json:"success"`
	Failed     []int64  `json:"failed"`
	Invoke     []int64  `json:"invoke"` // Invoke 写链调用量
	Query      []int64  `json:"query"`  // Query 读链调用量
	QuotaLimit int64    `json:"quota_limit"`
	QuotaUsed  int64    `json:"quota_used"`
}

// GetUsageStatsTrend 获取租户用量统计趋势（按天分组）
func (s *Service) GetUsageStatsTrend(ctx context.Context, tenantID uint, days int) (*UsageStatsTrend, error) {
	now := s.now()
	startTime := time.Date(now.Year(), now.Month(), now.Day()-days+1, 0, 0, 0, 0, s.loc)
	endTime := time.Date(now.Year(), now.Month(), now.Day()+1, 0, 0, 0, 0, s.loc)

	// 查询数据库中的按天统计
	dailyStats, err := s.repo.GetDailyUsageStats(ctx, tenantID, startTime, endTime)
	if err != nil {
		return nil, err
	}

	// 构建日期到统计的映射
	// 注意：GORM Scan 可能将 DATE(created_at) 解析为完整时间格式如 "2026-06-08T00:00:00+08:00"
	// 需要统一转换为 YYYY-MM-DD 格式作为 map key
	statMap := make(map[string]*store.DailyUsageStat, len(dailyStats))
	for _, stat := range dailyStats {
		// 尝试解析 Date 字段，统一转为 YYYY-MM-DD 格式
		dateKey := stat.Date
		if t, err := time.Parse("2006-01-02T15:04:05Z07:00", stat.Date); err == nil {
			dateKey = t.Format("2006-01-02")
		} else if t, err := time.Parse("2006-01-02", stat.Date); err == nil {
			dateKey = t.Format("2006-01-02")
		}
		statMap[dateKey] = stat
	}

	// 查询 Invoke/Query 分类统计
	invokeDailyStats, err := s.repo.GetDailyUsageStatsByMethodType(ctx, tenantID, startTime, endTime)
	if err != nil {
		return nil, err
	}

	// 构建 Invoke 日期到统计的映射
	invokeStatMap := make(map[string]*store.DailyUsageStat, len(invokeDailyStats))
	for _, stat := range invokeDailyStats {
		dateKey := stat.Date
		if t, err := time.Parse("2006-01-02T15:04:05Z07:00", stat.Date); err == nil {
			dateKey = t.Format("2006-01-02")
		} else if t, err := time.Parse("2006-01-02", stat.Date); err == nil {
			dateKey = t.Format("2006-01-02")
		}
		invokeStatMap[dateKey] = stat
	}

	// Query 类型暂时不查库，默认为 0（后续可扩展）
	// 当前 GetDailyUsageStatsByMethodType 仅查询 Invoke 统计
	// Query 统计 = 总调用 - Invoke 调用

	// 生成完整的日期序列（填充无数据的天）
	trend := &UsageStatsTrend{
		Dates:   make([]string, 0, days),
		Calls:   make([]int64, 0, days),
		Success: make([]int64, 0, days),
		Failed:  make([]int64, 0, days),
		Invoke:  make([]int64, 0, days),
		Query:   make([]int64, 0, days),
	}

	for i := 0; i < days; i++ {
		d := startTime.AddDate(0, 0, i)
		dateStr := d.Format("2006-01-02")
		// 前端显示用 MM/DD 格式
		displayDate := d.Format("1/2")

		trend.Dates = append(trend.Dates, displayDate)

		if stat, ok := statMap[dateStr]; ok {
			trend.Calls = append(trend.Calls, stat.Total)
			trend.Success = append(trend.Success, stat.Success)
			trend.Failed = append(trend.Failed, stat.Failed)
		} else {
			trend.Calls = append(trend.Calls, 0)
			trend.Success = append(trend.Success, 0)
			trend.Failed = append(trend.Failed, 0)
		}

		// Invoke/Query 分类统计
		if invokeStat, ok := invokeStatMap[dateStr]; ok {
			trend.Invoke = append(trend.Invoke, invokeStat.Total)
		} else {
			trend.Invoke = append(trend.Invoke, 0)
		}
		// Query = 总调用 - Invoke
		var totalCalls int64
		if stat, ok := statMap[dateStr]; ok {
			totalCalls = stat.Total
		}
		var invokeCalls int64
		if invokeStat, ok := invokeStatMap[dateStr]; ok {
			invokeCalls = invokeStat.Total
		}
		queryCalls := totalCalls - invokeCalls
		if queryCalls < 0 {
			queryCalls = 0
		}
		trend.Query = append(trend.Query, queryCalls)
	}

	// 配额信息
	quota, err := s.repo.GetQuotaByTenant(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	if quota != nil {
		trend.QuotaLimit = int64(quota.MonthlyLimit)
		trend.QuotaUsed = int64(quota.MonthlyUsed)
	}

	return trend, nil
}

// ========== 内部方法 ==========

// RealtimeCost 实时计费信息
type RealtimeCost struct {
	Plan          string  `json:"plan"`           // 当前套餐
	MonthCalls    int64   `json:"month_calls"`    // 本月 Invoke 调用量（计费口径）
	MonthlyLimit  int64   `json:"monthly_limit"`  // 月配额上限
	MonthlyUsed   int64   `json:"monthly_used"`   // 月已用量
	CurrentCost   float64 `json:"current_cost"`   // 实时费用（元）
	Currency      string  `json:"currency"`       // 币种
	CostBreakdown string  `json:"cost_breakdown"` // 费用说明（如"免费额度内"、"超出 1000 次按 ¥0.01/次"）
	UsagePercent  float64 `json:"usage_percent"`  // 用量百分比
	CalculatedAt  string  `json:"calculated_at"`  // 计算时间
}

// GetRealtimeCost 获取租户实时计费信息（用户主动触发，与定时任务不冲突）
func (s *Service) GetRealtimeCost(ctx context.Context, tenantID uint, plan string) (*RealtimeCost, error) {
	now := time.Now()

	// 本月 Invoke 调用量（计费口径，与定时任务一致）
	monthCalls, err := s.repo.CountCallsByTenantMonth(ctx, tenantID, now.Year(), now.Month())
	if err != nil {
		return nil, fmt.Errorf("count month calls: %w", err)
	}

	// 配额信息
	quota, err := s.repo.GetQuotaByTenant(ctx, tenantID)
	if err != nil {
		return nil, fmt.Errorf("get quota: %w", err)
	}

	// 计算实时费用（与定时任务使用同一 calculateAmount 函数，确保一致性）
	currentCost := calculateAmount(plan, uint64(monthCalls))

	// 费用说明
	breakdown := getCostBreakdown(plan, uint64(monthCalls))

	// 配额与用量
	var monthlyLimit int64
	var monthlyUsed int64
	var usagePercent float64
	if quota != nil {
		monthlyLimit = int64(quota.MonthlyLimit)
		monthlyUsed = int64(quota.MonthlyUsed)
		if quota.MonthlyLimit > 0 {
			usagePercent = math.Round(float64(monthlyUsed)/float64(quota.MonthlyLimit)*100*10000) / 10000
		}
	}

	return &RealtimeCost{
		Plan:          plan,
		MonthCalls:    monthCalls,
		MonthlyLimit:  monthlyLimit,
		MonthlyUsed:   monthlyUsed,
		CurrentCost:   currentCost,
		Currency:      "CNY",
		CostBreakdown: breakdown,
		UsagePercent:  usagePercent,
		CalculatedAt:  now.Format("2006-01-02 15:04:05"),
	}, nil
}

// getCostBreakdown 根据套餐和调用量生成费用说明
func getCostBreakdown(plan string, totalCalls uint64) string {
	switch plan {
	case "free":
		if totalCalls <= 1000 {
			return "免费套餐，1000 次/月免费额度内"
		}
		return fmt.Sprintf("免费套餐，超出 1000 次部分按 ¥0.01/次计费（超出 %d 次）", totalCalls-1000)
	case "developer":
		if totalCalls <= 50000 {
			return "开发者版，月费 ¥99 含 50000 次调用"
		}
		return fmt.Sprintf("开发者版，月费 ¥99 + 超出 50000 次部分按 ¥0.005/次计费（超出 %d 次）", totalCalls-50000)
	case "enterprise":
		return "企业版，月费 ¥999，无限调用"
	default:
		return fmt.Sprintf("按量计费 ¥0.01/次")
	}
}

// getDailyCount 获取租户今日调用次数（原子计数，跨天自动隔离）
func (s *Service) getDailyCount(ctx context.Context, tenantID uint) (int64, error) {
	key := dailyKey(tenantID)
	// 快速路径：内存中已有当日计数
	if v, ok := s.dailyCounters.Load(key); ok {
		return v.(*atomic.Int64).Load(), nil
	}

	// 慢路径：从数据库回填当日调用次数（当日的历史累计）
	count, err := s.repo.CountCallsByTenantToday(ctx, tenantID)
	if err != nil {
		return 0, err
	}

	counter := new(atomic.Int64)
	counter.Store(count)
	// LoadOrStore 保证并发下唯一 counter 实例
	actual, loaded := s.dailyCounters.LoadOrStore(key, counter)
	if loaded {
		return actual.(*atomic.Int64).Load(), nil
	}
	return count, nil
}

// incrementDailyCount 原子地为当日计数 +1
func (s *Service) incrementDailyCount(tenantID uint) {
	key := dailyKey(tenantID)
	if v, ok := s.dailyCounters.Load(key); ok {
		v.(*atomic.Int64).Add(1)
		return
	}
	counter := new(atomic.Int64)
	counter.Store(1)
	actual, loaded := s.dailyCounters.LoadOrStore(key, counter)
	if loaded {
		actual.(*atomic.Int64).Add(1)
	}
}

// generateBillForTenant 为单个租户生成账单
func (s *Service) generateBillForTenant(ctx context.Context, tenantID uint, plan string,
	periodStart, periodEnd time.Time, billType string, totalCalls int64) error {

	if totalCalls == 0 {
		return nil // 无调用不生成账单
	}

	// 计算费用
	amount := calculateAmount(plan, uint64(totalCalls))

	bill := &store.Bill{
		TenantID:    tenantID,
		BillType:    billType,
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
