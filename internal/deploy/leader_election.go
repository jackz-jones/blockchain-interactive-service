package deploy

import (
	"context"
	"fmt"
	"time"

	"github.com/zeromicro/go-zero/core/logx"
)

// LeaderElection 基于 Redis 的分布式 Leader 选举
// 用于多实例环境下确保订阅任务只有一个实例执行
type LeaderElection struct {
	// redisClient Redis 客户端接口
	redisClient RedisLocker

	// key Leader 选举的 Redis key
	key string

	// instanceID 当前实例唯一标识
	instanceID string

	// ttl Leader 锁的 TTL
	ttl time.Duration

	// renewInterval 续约间隔
	renewInterval time.Duration

	// isLeader 当前是否为 Leader
	isLeader bool

	// onBecomeLeader 成为 Leader 时的回调
	onBecomeLeader func()

	// onLoseLeadership 失去 Leader 时的回调
	onLoseLeadership func()

	logger logx.Logger
}

// RedisLocker Redis 分布式锁接口
type RedisLocker interface {
	// SetNX 设置值（仅当 key 不存在时）
	SetNX(ctx context.Context, key, value string, ttl time.Duration) (bool, error)

	// Get 获取值
	Get(ctx context.Context, key string) (string, error)

	// Expire 设置过期时间
	Expire(ctx context.Context, key string, ttl time.Duration) error

	// Del 删除 key
	Del(ctx context.Context, key string) error
}

// LeaderElectionConfig Leader 选举配置
type LeaderElectionConfig struct {
	Key            string        // Redis key 前缀
	InstanceID     string        // 实例 ID
	TTL            time.Duration // 锁 TTL
	RenewInterval  time.Duration // 续约间隔
	OnBecomeLeader func()        // 成为 Leader 回调
	OnLoseLeader   func()        // 失去 Leader 回调
}

// NewLeaderElection 创建 Leader 选举实例
func NewLeaderElection(client RedisLocker, cfg *LeaderElectionConfig, logger logx.Logger) *LeaderElection {
	if cfg.TTL == 0 {
		cfg.TTL = 15 * time.Second
	}
	if cfg.RenewInterval == 0 {
		cfg.RenewInterval = 5 * time.Second
	}

	return &LeaderElection{
		redisClient:      client,
		key:              cfg.Key,
		instanceID:       cfg.InstanceID,
		ttl:              cfg.TTL,
		renewInterval:    cfg.RenewInterval,
		onBecomeLeader:   cfg.OnBecomeLeader,
		onLoseLeadership: cfg.OnLoseLeader,
		logger:           logger,
	}
}

// Start 启动 Leader 选举循环
func (le *LeaderElection) Start(ctx context.Context) {
	ticker := time.NewTicker(le.renewInterval)
	defer ticker.Stop()

	le.logger.Infof("[LeaderElection] started, instance=%s, key=%s", le.instanceID, le.key)

	for {
		select {
		case <-ctx.Done():
			le.release(context.Background())
			le.logger.Infof("[LeaderElection] stopped, instance=%s", le.instanceID)
			return
		case <-ticker.C:
			le.tryAcquireOrRenew(ctx)
		}
	}
}

// tryAcquireOrRenew 尝试获取或续约 Leader 锁
func (le *LeaderElection) tryAcquireOrRenew(ctx context.Context) {
	if le.isLeader {
		// 续约
		if err := le.renew(ctx); err != nil {
			le.logger.Errorf("[LeaderElection] renew failed: %v", err)
			le.isLeader = false
			if le.onLoseLeadership != nil {
				le.onLoseLeadership()
			}
		}
	} else {
		// 尝试获取
		acquired, err := le.acquire(ctx)
		if err != nil {
			le.logger.Errorf("[LeaderElection] acquire failed: %v", err)
			return
		}
		if acquired {
			le.isLeader = true
			le.logger.Infof("[LeaderElection] became leader, instance=%s", le.instanceID)
			if le.onBecomeLeader != nil {
				le.onBecomeLeader()
			}
		}
	}
}

// acquire 尝试获取 Leader 锁
func (le *LeaderElection) acquire(ctx context.Context) (bool, error) {
	ok, err := le.redisClient.SetNX(ctx, le.key, le.instanceID, le.ttl)
	if err != nil {
		return false, fmt.Errorf("setnx: %w", err)
	}
	return ok, nil
}

// renew 续约 Leader 锁
func (le *LeaderElection) renew(ctx context.Context) error {
	// 检查当前 Leader 是否是自己
	val, err := le.redisClient.Get(ctx, le.key)
	if err != nil {
		return fmt.Errorf("get: %w", err)
	}
	if val != le.instanceID {
		return fmt.Errorf("leader changed to %s", val)
	}

	// 续约
	if err := le.redisClient.Expire(ctx, le.key, le.ttl); err != nil {
		return fmt.Errorf("expire: %w", err)
	}
	return nil
}

// release 释放 Leader 锁
func (le *LeaderElection) release(ctx context.Context) {
	if !le.isLeader {
		return
	}

	// 只有当前 Leader 才能释放
	val, err := le.redisClient.Get(ctx, le.key)
	if err != nil || val != le.instanceID {
		return
	}

	_ = le.redisClient.Del(ctx, le.key)
	le.isLeader = false
	le.logger.Infof("[LeaderElection] released leadership, instance=%s", le.instanceID)
}

// IsLeader 返回当前实例是否为 Leader
func (le *LeaderElection) IsLeader() bool {
	return le.isLeader
}
