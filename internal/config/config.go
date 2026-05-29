package config

import (
	"fmt"
	"strings"
	"time"

	"github.com/spf13/viper"
)

// Config is the root configuration structure for VmOrbit.
type Config struct {
	Server     ServerConfig     `mapstructure:"server"`
	Database   DatabaseConfig   `mapstructure:"database"`
	Redis      RedisConfig      `mapstructure:"redis"`
	JWT        JWTConfig        `mapstructure:"jwt"`
	Providers  ProvidersConfig  `mapstructure:"providers"`
	TaskEngine TaskEngineConfig `mapstructure:"task_engine"`
	Log        LogConfig        `mapstructure:"log"`
	Metrics    MetricsConfig    `mapstructure:"metrics"`
}

type ServerConfig struct {
	Port         int           `mapstructure:"port"`
	ReadTimeout  time.Duration `mapstructure:"read_timeout"`
	WriteTimeout time.Duration `mapstructure:"write_timeout"`
	IdleTimeout  time.Duration `mapstructure:"idle_timeout"`
	Mode         string        `mapstructure:"mode"` // debug | release
	CORSOrigins  []string      `mapstructure:"cors_origins"`
}

type DatabaseConfig struct {
	Host            string        `mapstructure:"host"`
	Port            int           `mapstructure:"port"`
	User            string        `mapstructure:"user"`
	Password        string        `mapstructure:"password"`
	DBName          string        `mapstructure:"dbname"`
	SSLMode         string        `mapstructure:"sslmode"`
	MaxOpenConns    int           `mapstructure:"max_open_conns"`
	MaxIdleConns    int           `mapstructure:"max_idle_conns"`
	ConnMaxLifetime time.Duration `mapstructure:"conn_max_lifetime"`
}

func (d DatabaseConfig) DSN() string {
	return fmt.Sprintf(
		"host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
		d.Host, d.Port, d.User, d.Password, d.DBName, d.SSLMode,
	)
}

type RedisConfig struct {
	Host     string `mapstructure:"host"`
	Port     int    `mapstructure:"port"`
	Password string `mapstructure:"password"`
	DB       int    `mapstructure:"db"`
	PoolSize int    `mapstructure:"pool_size"`
}

func (r RedisConfig) Addr() string {
	return fmt.Sprintf("%s:%d", r.Host, r.Port)
}

type JWTConfig struct {
	Secret          string        `mapstructure:"secret"`
	AccessTokenTTL  time.Duration `mapstructure:"access_token_ttl"`
	RefreshTokenTTL time.Duration `mapstructure:"refresh_token_ttl"`
	Issuer          string        `mapstructure:"issuer"`
}

type ProvidersConfig struct {
	VMware  VMwareConfig  `mapstructure:"vmware"`
	Proxmox ProxmoxConfig `mapstructure:"proxmox"`
	Nutanix NutanixConfig `mapstructure:"nutanix"`
}

type NutanixConfig struct {
	Enabled        bool          `mapstructure:"enabled"`
	DefaultTimeout time.Duration `mapstructure:"default_timeout"`
	VerifyTLS      bool          `mapstructure:"verify_tls"`
}

type VMwareConfig struct {
	Enabled            bool          `mapstructure:"enabled"`
	DefaultTimeout     time.Duration `mapstructure:"default_timeout"`
	MaxConcurrentConns int           `mapstructure:"max_concurrent_conns"`
}

type ProxmoxConfig struct {
	Enabled        bool          `mapstructure:"enabled"`
	DefaultTimeout time.Duration `mapstructure:"default_timeout"`
	VerifyTLS      bool          `mapstructure:"verify_tls"`
}

type TaskEngineConfig struct {
	WorkerCount     int           `mapstructure:"worker_count"`
	QueueSize       int           `mapstructure:"queue_size"`
	MaxRetries      int           `mapstructure:"max_retries"`
	RetryBaseDelay  time.Duration `mapstructure:"retry_base_delay"`  // base for exponential backoff
	TaskTTL         time.Duration `mapstructure:"task_ttl"`
	PollInterval    time.Duration `mapstructure:"poll_interval"`     // DB fallback poll interval
	LockTTL         time.Duration `mapstructure:"lock_ttl"`          // Redis worker lock TTL
	MaxLogEntries   int           `mapstructure:"max_log_entries"`   // max Redis log entries per task
	DefaultTimeout  time.Duration `mapstructure:"default_timeout"`   // per-task execution timeout
}

type LogConfig struct {
	Level  string `mapstructure:"level"`  // debug | info | warn | error
	Format string `mapstructure:"format"` // json | console
}

type MetricsConfig struct {
	Enabled bool   `mapstructure:"enabled"`
	Path    string `mapstructure:"path"`
}

// Load reads configuration from file and environment variables.
func Load() (*Config, error) {
	v := viper.New()

	// Defaults
	setDefaults(v)

	// Config file
	v.SetConfigName("config")
	v.SetConfigType("yaml")
	v.AddConfigPath("./configs")
	v.AddConfigPath(".")

	if err := v.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return nil, fmt.Errorf("reading config file: %w", err)
		}
	}

	// Environment variables override (VMOORBIT_SERVER_PORT, etc.)
	v.SetEnvPrefix("VMORBIT")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("unmarshalling config: %w", err)
	}

	return &cfg, nil
}

func setDefaults(v *viper.Viper) {
	v.SetDefault("server.port", 8080)
	v.SetDefault("server.read_timeout", "15s")
	v.SetDefault("server.write_timeout", "15s")
	v.SetDefault("server.idle_timeout", "60s")
	v.SetDefault("server.mode", "release")

	v.SetDefault("database.host", "localhost")
	v.SetDefault("database.port", 5432)
	v.SetDefault("database.sslmode", "disable")
	v.SetDefault("database.max_open_conns", 25)
	v.SetDefault("database.max_idle_conns", 10)
	v.SetDefault("database.conn_max_lifetime", "5m")

	v.SetDefault("redis.host", "localhost")
	v.SetDefault("redis.port", 6379)
	v.SetDefault("redis.db", 0)
	v.SetDefault("redis.pool_size", 10)

	v.SetDefault("jwt.access_token_ttl", "15m")
	v.SetDefault("jwt.refresh_token_ttl", "168h")
	v.SetDefault("jwt.issuer", "vmOrbit")

	v.SetDefault("task_engine.worker_count", 10)
	v.SetDefault("task_engine.queue_size", 1000)
	v.SetDefault("task_engine.max_retries", 3)
	v.SetDefault("task_engine.retry_base_delay", "5s")
	v.SetDefault("task_engine.task_ttl", "24h")
	v.SetDefault("task_engine.poll_interval", "5s")
	v.SetDefault("task_engine.lock_ttl", "300s")
	v.SetDefault("task_engine.max_log_entries", 500)
	v.SetDefault("task_engine.default_timeout", "10m")

	v.SetDefault("log.level", "info")
	v.SetDefault("log.format", "json")

	v.SetDefault("metrics.enabled", true)
	v.SetDefault("metrics.path", "/metrics")

	// Provider defaults
	v.SetDefault("providers.nutanix.enabled", true)
	v.SetDefault("providers.nutanix.default_timeout", "30s")
	v.SetDefault("providers.nutanix.verify_tls", false)
}
