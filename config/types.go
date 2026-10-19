package config

import "time"

type Config struct {
	Server *ServerConfig
	Log    *LogConfig
	Secret []*SecretConfig
	Store  *StoreConfig
}

type ServerConfig struct {
	ID   string // 服务 id
	Host string // 服务地址
	Port int    // 监听端口
	Salt string // token 密钥
}

type LogConfig struct {
	Path   string // 文件位置
	Level  string // 日志等级
	Size   int    // 日志大小限制
	Backup int    // 最大备份数量
}

type SecretConfig struct {
	Version int    // 密钥版本
	Key     string // 密钥
}

type StoreConfig struct {
	DB              string
	Dsn             string
	MaxIdleConn     int
	MinIdleConn     int
	MaxConnIdleTime time.Duration
	Debug           bool
}
