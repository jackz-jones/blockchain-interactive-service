package store

import (
	"fmt"

	"github.com/zeromicro/go-zero/core/logx"
	"gorm.io/driver/mysql"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// DBConfig 数据库配置
type DBConfig struct {
	Driver   string // 数据库驱动：postgres、mysql
	Host     string // 主机地址
	Port     int    // 端口
	User     string // 用户名
	Password string // 密码
	DBName   string // 数据库名
	SSLMode  string // SSL 模式（postgres 专用）
}

// DSN 生成数据库连接字符串
func (c *DBConfig) DSN() string {
	switch c.Driver {
	case "postgres":
		sslMode := c.SSLMode
		if sslMode == "" {
			sslMode = "disable"
		}
		return fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
			c.Host, c.Port, c.User, c.Password, c.DBName, sslMode)
	case "mysql":
		return fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?charset=utf8mb4&parseTime=True&loc=Local",
			c.User, c.Password, c.Host, c.Port, c.DBName)
	default:
		return ""
	}
}

// NewDB 创建数据库连接
func NewDB(cfg *DBConfig) (*gorm.DB, error) {
	var dialector gorm.Dialector

	switch cfg.Driver {
	case "postgres":
		dialector = postgres.Open(cfg.DSN())
	case "mysql":
		dialector = mysql.Open(cfg.DSN())
	default:
		return nil, fmt.Errorf("unsupported database driver: %s", cfg.Driver)
	}

	db, err := gorm.Open(dialector, &gorm.Config{
		Logger: logger.Default.LogMode(logger.Warn),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to connect database: %w", err)
	}

	logx.Infof("[Store] database connected successfully, driver=%s, host=%s, db=%s",
		cfg.Driver, cfg.Host, cfg.DBName)

	return db, nil
}

// AutoMigrate 自动迁移数据库表结构
func AutoMigrate(db *gorm.DB) error {
	err := db.AutoMigrate(
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
		return fmt.Errorf("failed to auto migrate: %w", err)
	}

	logx.Info("[Store] database auto migration completed")
	return nil
}
