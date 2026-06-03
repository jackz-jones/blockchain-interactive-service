package store

import (
	"fmt"

	"github.com/jackz-jones/blockchain-interactive-service/internal/config"
	commonDB "github.com/jackz-jones/common/db"
	"github.com/zeromicro/go-zero/core/logx"
	"gorm.io/gorm"
)

// NewDB 创建数据库连接并自动迁移表结构
func NewDB(cfg *config.DatabaseConf) (*gorm.DB, error) {
	if cfg.DSN == "" {
		return nil, fmt.Errorf("database DSN is required")
	}

	// 需要迁移的表模型列表
	models := []interface{}{
		&Tenant{},
		&User{},
		&APIKey{},
		&TenantChainConfig{},
		&TenantChainNode{},
		&TenantContractConfig{},
		&CallLog{},
		&Bill{},
		&Quota{},
		&AuditLog{},
	}

	// 自定义 gorm 配置：禁止 AutoMigrate 时使用外键约束操作来处理索引变更，
	// 避免 MySQL 报 Error 1091: Can't DROP ... check that column/key exists
	gormConf := &gorm.Config{
		DisableForeignKeyConstraintWhenMigrating: true,
	}

	db, err := commonDB.InitGormDB(cfg.Type, cfg.DSN, models, gormConf)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize database: %w", err)
	}

	logx.Infof("[Store] database connected successfully, type=%s", cfg.Type)

	return db, nil
}
