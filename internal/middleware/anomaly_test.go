package middleware

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/jackz-jones/blockchain-interactive-service/internal/store"
	"github.com/zeromicro/go-zero/core/logx"
)

// fakeRepoForAnomaly 仅实现 UpdateAPIKeyStatusByID，其它方法 panic
type fakeRepoForAnomaly struct {
	store.Repository
	mu       sync.Mutex
	statuses map[uint]string
	calls    int
}

func (f *fakeRepoForAnomaly) UpdateAPIKeyStatusByID(_ context.Context, id uint, status string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.statuses == nil {
		f.statuses = make(map[uint]string)
	}
	f.statuses[id] = status
	f.calls++
	return nil
}

func TestAnomalyDetector_RecordFailure_BanFlow(t *testing.T) {
	repo := &fakeRepoForAnomaly{}
	cfg := &AnomalyDetectorConfig{
		WindowDuration: 500 * time.Millisecond,
		MaxFailures:    3,
		BanDuration:    time.Minute,
	}
	d := NewAnomalyDetector(repo, cfg, logx.WithContext(context.Background()))
	defer d.Stop()

	// 累计 3 次失败即封禁
	for i := 0; i < 3; i++ {
		d.RecordFailure(42)
	}

	if !d.IsKeyBanned(42) {
		t.Fatal("key should be banned after reaching maxFailures")
	}

	// 等待异步落库
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		repo.mu.Lock()
		status := repo.statuses[42]
		repo.mu.Unlock()
		if status == "revoked" {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("expected DB status to be updated to revoked")
}

func TestAnomalyDetector_ExpiredFailuresDontTriggerBan(t *testing.T) {
	repo := &fakeRepoForAnomaly{}
	cfg := &AnomalyDetectorConfig{
		WindowDuration: 200 * time.Millisecond,
		MaxFailures:    3,
		BanDuration:    time.Minute,
	}
	d := NewAnomalyDetector(repo, cfg, logx.WithContext(context.Background()))
	defer d.Stop()

	d.RecordFailure(7)
	d.RecordFailure(7)
	time.Sleep(300 * time.Millisecond) // 窗口过期
	d.RecordFailure(7)

	if d.IsKeyBanned(7) {
		t.Fatal("expired failures must not accumulate into a ban")
	}
}
