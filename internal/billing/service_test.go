package billing

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jackz-jones/blockchain-interactive-service/internal/store"
	"github.com/zeromicro/go-zero/core/logx"
)

// mockRepo 仅实现 CheckQuota / getDailyCount / RecordUsage 用到的方法，
// 其余方法通过嵌入 store.Repository 空接口而未实现——测试路径不会触发这些方法。
type mockRepo struct {
	store.Repository

	mu             sync.Mutex
	quota          *store.Quota
	todayCount     int64
	monthlyUsedInc uint64
}

func (m *mockRepo) GetQuotaByTenant(_ context.Context, _ uint) (*store.Quota, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.quota == nil {
		return nil, nil
	}
	// 返回副本以避免测试中被修改
	cp := *m.quota
	return &cp, nil
}

func (m *mockRepo) CountCallsByTenantToday(_ context.Context, _ uint) (int64, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.todayCount, nil
}

func (m *mockRepo) IncrementMonthlyUsed(_ context.Context, _ uint, delta uint64) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.monthlyUsedInc += delta
	return nil
}

func newTestService(repo store.Repository) *Service {
	// 手动构造，避免启动后台 GC 协程干扰测试
	return &Service{
		repo:   repo,
		logger: logx.WithContext(context.Background()),
		stopCh: make(chan struct{}),
	}
}

func TestCheckQuota_NoQuotaRecord_Allow(t *testing.T) {
	svc := newTestService(&mockRepo{})
	d, err := svc.CheckQuota(context.Background(), 1)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if !d.Allowed {
		t.Fatalf("expected allowed=true, got %+v", d)
	}
}

func TestCheckQuota_DailyLimit_BlockPolicy(t *testing.T) {
	repo := &mockRepo{
		quota:      &store.Quota{DailyLimit: 10, OveragePolicy: "block"},
		todayCount: 10,
	}
	svc := newTestService(repo)

	d, err := svc.CheckQuota(context.Background(), 1)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if d.Allowed || d.Throttled {
		t.Fatalf("expected block: allowed=false throttled=false, got %+v", d)
	}
}

func TestCheckQuota_DailyLimit_ThrottlePolicy(t *testing.T) {
	repo := &mockRepo{
		quota:      &store.Quota{DailyLimit: 10, OveragePolicy: "throttle"},
		todayCount: 10,
	}
	svc := newTestService(repo)

	d, err := svc.CheckQuota(context.Background(), 1)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if d.Allowed || !d.Throttled {
		t.Fatalf("expected throttle: allowed=false throttled=true, got %+v", d)
	}
}

func TestCheckQuota_MonthlyWarning(t *testing.T) {
	repo := &mockRepo{
		// 90% 已用，触发 80% warning 线
		quota: &store.Quota{MonthlyLimit: 100, MonthlyUsed: 90},
	}
	svc := newTestService(repo)

	d, err := svc.CheckQuota(context.Background(), 1)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if !d.Allowed || !d.Warning {
		t.Fatalf("expected allowed=true warning=true, got %+v", d)
	}
}

func TestIncrementDailyCount_ConcurrentAtomic(t *testing.T) {
	svc := newTestService(&mockRepo{})
	var wg sync.WaitGroup
	const N = 500
	for i := 0; i < N; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			svc.incrementDailyCount(42)
		}()
	}
	wg.Wait()

	key := dailyKey(42)
	v, ok := svc.dailyCounters.Load(key)
	if !ok {
		t.Fatalf("expected counter to exist for key %s", key)
	}
	got := v.(*atomic.Int64).Load()
	if got != int64(N) {
		t.Fatalf("expected %d, got %d", N, got)
	}
}

func TestDailyKey_IsolatedByDate(t *testing.T) {
	// 通过手工造两个不同日期的 key 断言互不影响
	svc := newTestService(&mockRepo{})
	todayKey := dailyKey(1)
	otherKey := fmt.Sprintf("%d:%s", 1, time.Now().AddDate(0, 0, -1).Format("2006-01-02"))

	c1 := new(atomic.Int64)
	c1.Store(3)
	svc.dailyCounters.Store(otherKey, c1)

	svc.incrementDailyCount(1)
	svc.incrementDailyCount(1)

	v, _ := svc.dailyCounters.Load(todayKey)
	if v.(*atomic.Int64).Load() != 2 {
		t.Fatalf("today counter should be 2, got %d", v.(*atomic.Int64).Load())
	}
	v2, _ := svc.dailyCounters.Load(otherKey)
	if v2.(*atomic.Int64).Load() != 3 {
		t.Fatalf("yesterday counter should stay 3, got %d", v2.(*atomic.Int64).Load())
	}
}

func TestRecordUsage_IncrementsMonthlyAndDaily(t *testing.T) {
	repo := &mockRepo{quota: &store.Quota{MonthlyLimit: 1000, DailyLimit: 100}}
	svc := newTestService(repo)

	for i := 0; i < 5; i++ {
		if err := svc.RecordUsage(context.Background(), 7); err != nil {
			t.Fatalf("record usage err: %v", err)
		}
	}
	if repo.monthlyUsedInc != 5 {
		t.Fatalf("expected monthly increment 5, got %d", repo.monthlyUsedInc)
	}
	v, ok := svc.dailyCounters.Load(dailyKey(7))
	if !ok {
		t.Fatalf("daily counter missing")
	}
	if v.(*atomic.Int64).Load() != 5 {
		t.Fatalf("expected daily count 5, got %d", v.(*atomic.Int64).Load())
	}
}
