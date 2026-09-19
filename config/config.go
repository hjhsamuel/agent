package config

import (
	"errors"
	"os"
	"strconv"
	"strings"

	"github.com/joho/godotenv"
	"github.com/sirupsen/logrus"
	"gopkg.in/natefinch/lumberjack.v2"
)

const globalEnvFile = ".env"

type Config struct {
	Server  ServerConfig
	Log     LogConfig
	Secret  []*SecretConfig
	Storage StorageConfig
}

type ServerConfig struct {
	Host string
	Port int
	Salt string
}

type LogConfig struct {
	Path     string
	Level    string
	MaxSize  int
	MaxRolls int
}

type SecretConfig struct {
	Version int
	Key     string
}

type StorageConfig struct {
	Debug       bool
	MaxIdleConn int
	MinIdleConn int
	MaxIdleTime string
	DB          string
	Dsn         string
}

var gConf *Config

func Init() error {
	_ = godotenv.Load(globalEnvFile)

	gConf = &Config{}
	if err := setServerConfig(gConf); err != nil {
		return err
	}
	if err := setLogConfig(gConf); err != nil {
		return err
	}
	if err := setSecretConfig(gConf); err != nil {
		return err
	}
	if err := setStorageConfig(gConf); err != nil {
		return err
	}

	if err := initLog(); err != nil {
		return err
	}

	return nil
}

func setServerConfig(c *Config) error {
	c.Server = ServerConfig{
		Host: GetEnv("AGENT_HOST", "0.0.0.0"),
		Salt: GetEnv("AGENT_SALT", "oC8qQ5dZ4tC2"),
	}

	portStr := GetEnv("AGENT_PORT", "8000")
	port, err := strconv.Atoi(portStr)
	if err != nil {
		return err
	}
	c.Server.Port = port

	return nil
}

func setLogConfig(c *Config) error {
	c.Log = LogConfig{
		Path:     GetEnv("AGENT_LOG_PATH", "/app/log/agent.log"),
		Level:    GetEnv("AGENT_LOG_LEVEL", "info"),
		MaxSize:  0,
		MaxRolls: 0,
	}

	sizeStr := GetEnv("AGENT_LOG_SIZE", "50")
	size, err := strconv.Atoi(sizeStr)
	if err != nil {
		return err
	}
	c.Log.MaxSize = size

	rollStr := GetEnv("AGENT_LOG_ROLLS", "3")
	rolls, err := strconv.Atoi(rollStr)
	if err != nil {
		return err
	}
	c.Log.MaxRolls = rolls

	return nil
}

const secretPrefix = "AGENT_SECRET_VER_"

func setSecretConfig(c *Config) error {
	items := make([]*SecretConfig, 0)
	for _, v := range os.Environ() {
		if !strings.HasPrefix(v, secretPrefix) {
			continue
		}
		key, val, ok := strings.Cut(v, "=")
		if !ok {
			continue
		}
		ver, err := strconv.Atoi(strings.TrimPrefix(key, secretPrefix))
		if err != nil {
			continue
		}
		items = append(items, &SecretConfig{
			Version: ver,
			Key:     val,
		})
	}
	if len(items) == 0 {
		return errors.New("no secret set")
	}

	c.Secret = items

	return nil
}

func setStorageConfig(c *Config) error {
	c.Storage = StorageConfig{
		MaxIdleTime: GetEnv("AGENT_MAX_IDLE_TIME", "5m"),
		DB:          GetEnv("AGENT_MONGO_DB", "agent"),
		Dsn:         GetEnv("AGENT_MONGO_DSN", ""),
	}

	debugStr := GetEnv("AGENT_MONGO_DEBUG", "false")
	debug, err := strconv.ParseBool(debugStr)
	if err != nil {
		return err
	}
	c.Storage.Debug = debug

	maxIdleConnStr := GetEnv("AGENT_MONGO_MAX_IDLE_CONN", "200")
	maxIdleConn, err := strconv.Atoi(maxIdleConnStr)
	if err != nil {
		return err
	}
	c.Storage.MaxIdleConn = maxIdleConn

	minIdleConnStr := GetEnv("AGENT_MONGO_MIN_IDLE_CONN", "3")
	minIdleConn, err := strconv.Atoi(minIdleConnStr)
	if err != nil {
		return err
	}
	c.Storage.MinIdleConn = minIdleConn

	return nil
}

func initLog() error {
	if level, err := logrus.ParseLevel(gConf.Log.Level); err != nil {
		return err
	} else {
		logrus.SetLevel(level)
	}

	if gConf.Log.Path != "" {
		logrus.SetOutput(&lumberjack.Logger{
			Filename:   gConf.Log.Path,
			MaxSize:    gConf.Log.MaxSize,
			MaxBackups: gConf.Log.MaxRolls,
			LocalTime:  true,
			Compress:   true,
		})
	}

	f := &logrus.TextFormatter{
		FullTimestamp:   true,
		TimestampFormat: "2006-01-02 15:04:05",
	}
	logrus.SetFormatter(f)

	return nil
}
