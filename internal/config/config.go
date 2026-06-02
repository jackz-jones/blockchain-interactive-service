package config

import (
	"github.com/zeromicro/go-zero/zrpc"
)

type Config struct {
	zrpc.RpcServerConf
	GrpcConf      GrpcConf
	SubscribeConf SubscribeConf

	// 数据库配置（多租户持久化存储）
	DatabaseConf DatabaseConf

	// HTTP Gateway 配置
	GatewayConf GatewayConf
}

// GatewayConf HTTP API Gateway 配置
type GatewayConf struct {
	// Enable 是否启用 HTTP Gateway
	Enable bool `json:",default=true"` //nolint:staticcheck

	// Host HTTP 监听地址
	Host string `json:",default=0.0.0.0"` //nolint:staticcheck

	// Port HTTP 监听端口
	Port int `json:",default=8080"` //nolint:staticcheck

	// RateLimit 默认 QPS 限制（每租户）
	RateLimit int `json:",default=10"` //nolint:staticcheck
}

// DatabaseConf 数据库配置
// 参考 common 包 db.InitGormDB，支持 mysql、kingbase_mysql、kingbase_pgsql、postgres
type DatabaseConf struct {
	// Type 数据库类型，支持: mysql、kingbase_mysql、kingbase_pgsql、postgres
	Type string `json:",default=mysql"` //nolint:staticcheck

	// DSN 数据库连接字符串
	DSN string

	// MaxIdleConns 空闲连接池最大数量（可选，默认使用 common 包内置值）
	MaxIdleConns int `json:",optional"` //nolint:staticcheck

	// MaxOpenConns 最大连接数（可选，默认使用 common 包内置值）
	MaxOpenConns int `json:",optional"` //nolint:staticcheck
}

// GrpcConf contain all config items for grpc server initiation
type GrpcConf struct {
	// CaCertFile 是 CA 根证书文件的路径
	CaCertFile string

	// ServerCertFile 是服务端证书文件的路径
	ServerCertFile string

	// ServerKeyFile 是服务端私钥文件的路径
	ServerKeyFile string

	// MaxRecvMsgSize 是最大接收消息大小
	MaxRecvMsgSize int

	// MaxSendMsgSize 是最大发送消息大小
	MaxSendMsgSize int
}

// SubscribeConf contain all config items for subscribing chain event
type SubscribeConf struct {

	// confType 配置类型（cluster或者node）
	ConfType string

	// RedisAddr 是 Redis 服务器地址
	RedisAddr string

	// RedisUserName 是 Redis 用户名
	RedisUserName string

	// RedisPassword 是 Redis 密码
	RedisPassword string

	// nolint:staticcheck
	// 哨兵模式的MasterName，其他模式可忽略
	MasterName string `json:",optional"`
}
