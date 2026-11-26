package middleware

import (
	"testing"
	"time"
)

func TestPruneExpired(t *testing.T) {
	now := time.Now()
	tests := []struct {
		name        string
		input       []time.Time
		windowStart time.Time
		wantLen     int
	}{
		{
			name:        "empty slice",
			input:       nil,
			windowStart: now,
			wantLen:     0,
		},
		{
			name: "all expired",
			input: []time.Time{
				now.Add(-3 * time.Second),
				now.Add(-2 * time.Second),
			},
			windowStart: now.Add(-time.Second),
			wantLen:     0,
		},
		{
			name: "none expired (first item already in window)",
			input: []time.Time{
				now.Add(-500 * time.Millisecond),
				now.Add(-300 * time.Millisecond),
			},
			windowStart: now.Add(-time.Second),
			wantLen:     2,
		},
		{
			name: "partial expired",
			input: []time.Time{
				now.Add(-3 * time.Second),
				now.Add(-2 * time.Second),
				now.Add(-500 * time.Millisecond),
				now.Add(-100 * time.Millisecond),
			},
			windowStart: now.Add(-time.Second),
			wantLen:     2,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := pruneExpired(tc.input, tc.windowStart)
			if len(got) != tc.wantLen {
				t.Fatalf("want len=%d, got=%d, result=%v", tc.wantLen, len(got), got)
			}
		})
	}
}

func TestRateLimiter_Allow(t *testing.T) {
	rl := NewRateLimiter(3)
	defer rl.Stop()

	// 前 3 次允许
	for i := 0; i < 3; i++ {
		if !rl.Allow(1, 3) {
			t.Fatalf("iteration %d should be allowed", i)
		}
	}
	// 第 4 次拒绝
	if rl.Allow(1, 3) {
		t.Fatal("4th request should be rejected")
	}

	// 等待窗口过期后应放行
	time.Sleep(1100 * time.Millisecond)
	if !rl.Allow(1, 3) {
		t.Fatal("after window expired, should be allowed")
	}
}

func TestRateLimiter_AllowZeroValueEdge(t *testing.T) {
	// 验证第一个请求即在窗口内时不会因原 Bug 被误伤
	rl := NewRateLimiter(2)
	defer rl.Stop()

	if !rl.Allow(2, 2) {
		t.Fatal("first request must be allowed")
	}
	if !rl.Allow(2, 2) {
		t.Fatal("second request must be allowed")
	}
	if rl.Allow(2, 2) {
		t.Fatal("third request must be rejected")
	}
}
