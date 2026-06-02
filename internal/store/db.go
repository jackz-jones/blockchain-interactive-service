package store

import (
	"fmt"

	"github.com/jackz-jones/blockchain-interactive-service/internal/config"
	commonDB "github.com/jackz-jones/common/db"
	"github.com/zeromicro/go-zero/core/logx"
	"gorm.io/gorm"
)

// NewDB 创建数据库连接并自动迁移表结构
// 复用 common 包 InitGormDB 方法，统一连接池配置和初始化逻辑
func NewDB(cfg *config.DatabaseConf) (*gorm.DB, error) {
	if cfg.DSN == "" {
		return nil, fmt.Errorf("database DSN is required")
	}

	db, err := commonDB.InitGormDB(cfg.Type, cfg.DSN,
		&Tenant{},
		&User{},
		&APIKey{},
		&TenantChainConfig{},
		&TenantContractConfig{},
		&CallLog{},
		&Bill{},
		&Quota{},
		&AuditLog{},
	)
	if err != nil {
		return nil, fmt.Errorf("failed to init database via common.InitGormDB: %w", err)
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

	logx.Infof("[Store] database connected successfully, type=%s", cfg.Type)

	return db, nil
}
