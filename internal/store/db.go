package store

import (
	"fmt"

	"github.com/jackz-jones/blockchain-interactive-service/internal/config"
	commonDB "github.com/jackz-jones/common/db"
	"github.com/zeromicro/go-zero/core/logx"
	"gorm.io/gorm"
)

// allModels 所有需要迁移的表模型列表（仅在 AutoMigrate 开启时使用）
var allModels = []interface{}{
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

// NewDB 创建数据库连接，根据配置决定是否自动迁移表结构
func NewDB(cfg *config.DatabaseConf) (*gorm.DB, error) {
	if cfg.DSN == "" {
		return nil, fmt.Errorf("database DSN is required")
	}

	// 传 nil models 给 InitGormDB，跳过其内部的 AutoMigrate
	db, err := commonDB.InitGormDB(cfg.Type, cfg.DSN, nil, &gorm.Config{})
	if err != nil {
		return nil, fmt.Errorf("failed to initialize database: %w", err)
	}

	// 仅在配置开启时执行 AutoMigrate（开发/测试环境首次建表使用，生产环境通过 migration 工具管理）
	if cfg.AutoMigrate {
		logx.Info("[Store] AutoMigrate enabled, migrating table schemas...")
		if migrateErr := db.AutoMigrate(allModels...); migrateErr != nil {
			return nil, fmt.Errorf("failed to auto migrate tables: %w", migrateErr)
		}
		logx.Info("[Store] AutoMigrate completed successfully")
	}

	// 如果用户配置了自定义连接池参数，覆盖 common 包的默认值
	if cfg.MaxIdleConns > 0 || cfg.MaxOpenConns > 0 {
		sqlDB, sqlErr := db.DB()
		if sqlErr == nil {
			if cfg.MaxIdleConns > 0 {
				sqlDB.SetMaxIdleConns(cfg.MaxIdleConns)
			}
			if cfg.MaxOpenConns > 0 {
				sqlDB.SetMaxOpenConns(cfg.MaxOpenConns)
			}
		}
	}

	logx.Infof("[Store] database connected successfully, type=%s, autoMigrate=%v", cfg.Type, cfg.AutoMigrate)

	return db, nil
}