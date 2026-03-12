// Package config provides centralized configuration management
// All paths and settings are loaded from environment variables for Docker compatibility
package config

import (
	"crypto/sha256"
	"net"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"golang.org/x/crypto/pbkdf2"
)

// Config holds all application configuration
type Config struct {
	// Base directories
	BaseDir  string // SAMPMAIL_KUMO_DIR (default: /opt/kumomta)
	DataDir  string // SAMPMAIL_DATA_DIR (default: /var/lib/sampmail)
	LogDir   string // SAMPMAIL_LOG_DIR (default: /var/log/kumomta)
	SpoolDir string // SAMPMAIL_SPOOL_DIR (default: /var/spool/kumomta)
	HomeDir  string // SAMPMAIL_HOME_DIR (default: /home)

	// Server settings
	ListenAddr string // SAMPMAIL_LISTEN_ADDR (default: 127.0.0.1:9000)
	SMTPAddr   string // SAMPMAIL_SMTP_ADDR (default: kumomta:587 for submission)

	// TLS Settings
	TLSEnabled  bool   // SAMPMAIL_TLS_ENABLED (default: false)
	TLSCertFile string // SAMPMAIL_TLS_CERT (path to certificate)
	TLSKeyFile  string // SAMPMAIL_TLS_KEY (path to key)

	// Security
	AppSecret string // SAMPMAIL_SECRET (required, min 32 chars)

	// Public-facing base URL for tracking/unsubscribe links.
	// Set via SAMPMAIL_BASE_URL (e.g. https://mail.yourdomain.com).
	// Auto-derived from listen address when not set.
	BaseURL string // SAMPMAIL_BASE_URL

	// Proxy settings for rate limiting
	TrustProxy       bool     // SAMPMAIL_TRUST_PROXY (default: false)
	TrustedProxyCIDR []string // SAMPMAIL_TRUSTED_PROXY_CIDR (comma-separated CIDRs)

	// Worker settings
	CampaignWorkers int // SAMPMAIL_CAMPAIGN_WORKERS (default: 50)
	ImportBatchSize int // SAMPMAIL_IMPORT_BATCH_SIZE (default: 500)

	// Feature flags
	FirewallEnabled bool // SAMPMAIL_FIREWALL_ENABLED (default: true)

	// Reacher (Email Verification) settings
	ReacherURL    string // REACHER_URL (default: http://reacher:8080)
	ReacherAPIKey string // REACHER_API_KEY (optional, for Reacher Cloud)

	// Database settings (PostgreSQL)
	DatabaseDriver   string // SAMPMAIL_DB_DRIVER (default: sqlite, options: sqlite, postgres)
	PostgresHost     string // SAMPMAIL_PG_HOST
	PostgresPort     int    // SAMPMAIL_PG_PORT (default: 5432)
	PostgresUser     string // SAMPMAIL_PG_USER
	PostgresPassword string // SAMPMAIL_PG_PASSWORD
	PostgresDatabase string // SAMPMAIL_PG_DATABASE
	PostgresSSLMode  string // SAMPMAIL_PG_SSLMODE (default: require)
	DBMaxOpenConns   int    // SAMPMAIL_DB_MAX_OPEN_CONNS (default: 100)
	DBMaxIdleConns   int    // SAMPMAIL_DB_MAX_IDLE_CONNS (default: 10)

	// Redis settings (for distributed rate limiting)
	RedisAddr     string // SAMPMAIL_REDIS_ADDR (optional, enables distributed rate limiting)
	RedisPassword string // SAMPMAIL_REDIS_PASSWORD
	RedisDB       int    // SAMPMAIL_REDIS_DB (default: 0)

	// KumoMTA API settings
	KumoAPIURL       string // SAMPMAIL_KUMO_API_URL (default: http://127.0.0.1:8000)
	UseKumoHTTPAPI   bool   // SAMPMAIL_USE_KUMO_HTTP (default: true - use HTTP API instead of SMTP)

	// Request settings
	RequestTimeout  time.Duration // SAMPMAIL_REQUEST_TIMEOUT (default: 30s)
	MaxRequestBody  int64         // SAMPMAIL_MAX_REQUEST_BODY (default: 10MB)
	ShutdownTimeout time.Duration // SAMPMAIL_SHUTDOWN_TIMEOUT (default: 30s)

	// SMTP Pool settings
	SMTPMaxConns    int           // SAMPMAIL_SMTP_MAX_CONNS (default: 100)
	SMTPMinConns    int           // SAMPMAIL_SMTP_MIN_CONNS (default: 5)
	SMTPConnTimeout time.Duration // SAMPMAIL_SMTP_CONN_TIMEOUT (default: 10s)

	// Backup settings
	BackupRetention int // SAMPMAIL_BACKUP_RETENTION (default: 3)

	// Logging
	LogLevel string // SAMPMAIL_LOG_LEVEL (default: info)
	LogJSON  bool   // SAMPMAIL_LOG_JSON (default: true in production)

	// Metrics
	MetricsEnabled bool   // SAMPMAIL_METRICS_ENABLED (default: true)
	MetricsPath    string // SAMPMAIL_METRICS_PATH (default: /metrics)

	// Password hashing
	BcryptCost int // SAMPMAIL_BCRYPT_COST (default: 12)

	// Version info
	Version   string
	BuildTime string
	GitCommit string
}

var (
	cfg     *Config
	cfgOnce sync.Once
)

// Get returns the singleton configuration instance
func Get() *Config {
	cfgOnce.Do(func() {
		cfg = &Config{
			// Base directories
			BaseDir:  getEnv("SAMPMAIL_KUMO_DIR", "/opt/kumomta"),
			DataDir:  getEnv("SAMPMAIL_DATA_DIR", "/var/lib/sampmail"),
			LogDir:   getEnv("SAMPMAIL_LOG_DIR", "/var/log/kumomta"),
			SpoolDir: getEnv("SAMPMAIL_SPOOL_DIR", "/var/spool/kumomta"),
			HomeDir:  getEnv("SAMPMAIL_HOME_DIR", "/home"),

			// Server settings
			ListenAddr: getEnv("SAMPMAIL_LISTEN_ADDR", "127.0.0.1:9000"),
			SMTPAddr:   getEnv("SAMPMAIL_SMTP_ADDR", "kumomta:587"),

			// TLS Settings
			TLSEnabled:  getEnvBool("SAMPMAIL_TLS_ENABLED", false),
			TLSCertFile: os.Getenv("SAMPMAIL_TLS_CERT"),
			TLSKeyFile:  os.Getenv("SAMPMAIL_TLS_KEY"),

			// Security
			AppSecret: os.Getenv("SAMPMAIL_SECRET"),
			BaseURL:   deriveBaseURL(),

			// Proxy settings
			TrustProxy:       getEnvBool("SAMPMAIL_TRUST_PROXY", false),
			TrustedProxyCIDR: getEnvList("SAMPMAIL_TRUSTED_PROXY_CIDR", nil),

			// Worker settings
			CampaignWorkers: getEnvInt("SAMPMAIL_CAMPAIGN_WORKERS", 50),
			ImportBatchSize: getEnvInt("SAMPMAIL_IMPORT_BATCH_SIZE", 500),

			// Feature flags
			FirewallEnabled: getEnvBool("SAMPMAIL_FIREWALL_ENABLED", true),

			// Reacher settings
			ReacherURL:    getEnv("REACHER_URL", "http://reacher:8080"),
			ReacherAPIKey: os.Getenv("REACHER_API_KEY"),

			// Database settings - parse DATABASE_URL if present
			DatabaseDriver:   parseDatabaseDriver(),
			PostgresHost:     parsePostgresHost(),
			PostgresPort:     parsePostgresPort(),
			PostgresUser:     parsePostgresUser(),
			PostgresPassword: parsePostgresPassword(),
			PostgresDatabase: parsePostgresDatabase(),
			PostgresSSLMode:  parsePostgresSSLMode(),
			DBMaxOpenConns:   getEnvInt("SAMPMAIL_DB_MAX_OPEN_CONNS", 100),
			DBMaxIdleConns:   getEnvInt("SAMPMAIL_DB_MAX_IDLE_CONNS", 10),

			// Redis settings
			RedisAddr:     os.Getenv("SAMPMAIL_REDIS_ADDR"),
			RedisPassword: os.Getenv("SAMPMAIL_REDIS_PASSWORD"),
			RedisDB:       getEnvInt("SAMPMAIL_REDIS_DB", 0),

			// KumoMTA API
			KumoAPIURL:       getEnv("SAMPMAIL_KUMO_API_URL", "http://127.0.0.1:8000"),
			UseKumoHTTPAPI:   getEnvBool("SAMPMAIL_USE_KUMO_HTTP", true), // Default to HTTP API

			// Request settings
			RequestTimeout:  getEnvDuration("SAMPMAIL_REQUEST_TIMEOUT", 30*time.Second),
			MaxRequestBody:  getEnvInt64("SAMPMAIL_MAX_REQUEST_BODY", 10*1024*1024), // 10MB
			ShutdownTimeout: getEnvDuration("SAMPMAIL_SHUTDOWN_TIMEOUT", 30*time.Second),

			// SMTP Pool settings
			SMTPMaxConns:    getEnvInt("SAMPMAIL_SMTP_MAX_CONNS", 100),
			SMTPMinConns:    getEnvInt("SAMPMAIL_SMTP_MIN_CONNS", 5),
			SMTPConnTimeout: getEnvDuration("SAMPMAIL_SMTP_CONN_TIMEOUT", 10*time.Second),

			// Backup settings
			BackupRetention: getEnvInt("SAMPMAIL_BACKUP_RETENTION", 3),

			// Logging
			LogLevel: getEnv("SAMPMAIL_LOG_LEVEL", "info"),
			LogJSON:  getEnvBool("SAMPMAIL_LOG_JSON", os.Getenv("SAMPMAIL_ENV") != "development"),

			// Metrics
			MetricsEnabled: getEnvBool("SAMPMAIL_METRICS_ENABLED", true),
			MetricsPath:    getEnv("SAMPMAIL_METRICS_PATH", "/metrics"),

			// Password hashing - use cost 12 for better security
			BcryptCost: getEnvInt("SAMPMAIL_BCRYPT_COST", 12),

			// Version info (set at build time)
			Version:   getEnv("SAMPMAIL_VERSION", "dev"),
			BuildTime: os.Getenv("SAMPMAIL_BUILD_TIME"),
			GitCommit: os.Getenv("SAMPMAIL_GIT_COMMIT"),
		}
	})
	return cfg
}

// DKIMPath returns the DKIM keys directory
func (c *Config) DKIMPath() string {
	return c.BaseDir + "/etc/dkim"
}

// PolicyPath returns the policy directory
func (c *Config) PolicyPath() string {
	return c.BaseDir + "/etc/policy"
}

// BackupPath returns the backup directory
func (c *Config) BackupPath() string {
	return c.DataDir + "/backups"
}

// DatabasePath returns the database file path
func (c *Config) DatabasePath() string {
	return c.DataDir + "/sampmail.db"
}

// Validate checks that required configuration is present
func (c *Config) Validate() error {
	if c.AppSecret == "" {
		return &ConfigError{Field: "SAMPMAIL_SECRET", Message: "is required but not set"}
	}
	if len(c.AppSecret) < 32 {
		return &ConfigError{Field: "SAMPMAIL_SECRET", Message: "must be at least 32 characters"}
	}

	// Validate TLS settings
	if c.TLSEnabled {
		if c.TLSCertFile == "" {
			return &ConfigError{Field: "SAMPMAIL_TLS_CERT", Message: "is required when TLS is enabled"}
		}
		if c.TLSKeyFile == "" {
			return &ConfigError{Field: "SAMPMAIL_TLS_KEY", Message: "is required when TLS is enabled"}
		}
	}

	// Validate PostgreSQL settings if using postgres driver
	if c.DatabaseDriver == "postgres" {
		if c.PostgresUser == "" {
			return &ConfigError{Field: "SAMPMAIL_PG_USER", Message: "is required for PostgreSQL"}
		}
		if c.PostgresPassword == "" {
			return &ConfigError{Field: "SAMPMAIL_PG_PASSWORD", Message: "is required for PostgreSQL"}
		}
		if c.PostgresDatabase == "" {
			return &ConfigError{Field: "SAMPMAIL_PG_DATABASE", Message: "is required for PostgreSQL"}
		}
	}

	return nil
}

// IsTrustedProxy checks if the given IP is from a trusted proxy
func (c *Config) IsTrustedProxy(ip string) bool {
	if !c.TrustProxy {
		return false
	}

	parsed := net.ParseIP(ip)
	if parsed == nil {
		return false
	}

	for _, cidr := range c.TrustedProxyCIDR {
		_, network, err := net.ParseCIDR(cidr)
		if err != nil {
			continue
		}
		if network.Contains(parsed) {
			return true
		}
	}

	return false
}

// IsProduction returns true if running in production mode
func (c *Config) IsProduction() bool {
	return os.Getenv("SAMPMAIL_ENV") == "production"
}

// GetConfig returns the singleton configuration (alias for Get)
func GetConfig() *Config {
	return Get()
}

// ReloadConfig forces reload of configuration on next Get() call
// This is used when runtime config is updated from the database
func ReloadConfig() {
	cfgOnce = sync.Once{}
}

// DeriveEncryptionKey derives a proper encryption key from the secret
// using PBKDF2 instead of simple truncation
func (c *Config) DeriveEncryptionKey() []byte {
	salt := []byte("sampmail-encryption-key-v1")
	return pbkdf2.Key([]byte(c.AppSecret), salt, 100000, 32, sha256.New)
}

// DeriveSessionKey derives a key for session token hashing
func (c *Config) DeriveSessionKey() []byte {
	salt := []byte("sampmail-session-key-v1")
	return pbkdf2.Key([]byte(c.AppSecret), salt, 100000, 32, sha256.New)
}

// ConfigError represents a configuration error
type ConfigError struct {
	Field   string
	Message string
}

func (e *ConfigError) Error() string {
	return e.Field + " " + e.Message
}

// Helper functions
func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if i, err := strconv.Atoi(value); err == nil {
			return i
		}
	}
	return defaultValue
}

func getEnvBool(key string, defaultValue bool) bool {
	if value := os.Getenv(key); value != "" {
		return strings.ToLower(value) == "true" || value == "1"
	}
	return defaultValue
}

func getEnvList(key string, defaultValue []string) []string {
	if value := os.Getenv(key); value != "" {
		parts := strings.Split(value, ",")
		result := make([]string, 0, len(parts))
		for _, p := range parts {
			if trimmed := strings.TrimSpace(p); trimmed != "" {
				result = append(result, trimmed)
			}
		}
		return result
	}
	return defaultValue
}

func getEnvDuration(key string, defaultValue time.Duration) time.Duration {
	if value := os.Getenv(key); value != "" {
		if d, err := time.ParseDuration(value); err == nil {
			return d
		}
	}
	return defaultValue
}

func getEnvInt64(key string, defaultValue int64) int64 {
	if value := os.Getenv(key); value != "" {
		if i, err := strconv.ParseInt(value, 10, 64); err == nil {
			return i
		}
	}
	return defaultValue
}

// DATABASE_URL parsing functions
// Supports: postgres://user:password@host:port/database?sslmode=disable

func parseDatabaseURL() string {
	return os.Getenv("DATABASE_URL")
}

func parseDatabaseDriver() string {
	// Check explicit driver setting first
	if driver := os.Getenv("SAMPMAIL_DB_DRIVER"); driver != "" {
		return driver
	}
	// Auto-detect from DATABASE_URL
	dbURL := parseDatabaseURL()
	if strings.HasPrefix(dbURL, "postgres://") || strings.HasPrefix(dbURL, "postgresql://") {
		return "postgres"
	}
	return "sqlite"
}

func parsePostgresHost() string {
	if host := os.Getenv("SAMPMAIL_PG_HOST"); host != "" {
		return host
	}
	dbURL := parseDatabaseURL()
	if dbURL == "" {
		return "localhost"
	}
	// Parse: postgres://user:pass@host:port/db
	dbURL = strings.TrimPrefix(dbURL, "postgres://")
	dbURL = strings.TrimPrefix(dbURL, "postgresql://")
	// Remove user:pass@ part
	if idx := strings.Index(dbURL, "@"); idx != -1 {
		dbURL = dbURL[idx+1:]
	}
	// Get host:port part
	if idx := strings.Index(dbURL, "/"); idx != -1 {
		dbURL = dbURL[:idx]
	}
	// Split host:port
	host, _, _ := net.SplitHostPort(dbURL)
	if host == "" {
		return dbURL // No port specified
	}
	return host
}

func parsePostgresPort() int {
	if port := os.Getenv("SAMPMAIL_PG_PORT"); port != "" {
		if p, err := strconv.Atoi(port); err == nil {
			return p
		}
	}
	dbURL := parseDatabaseURL()
	if dbURL == "" {
		return 5432
	}
	// Parse: postgres://user:pass@host:port/db
	dbURL = strings.TrimPrefix(dbURL, "postgres://")
	dbURL = strings.TrimPrefix(dbURL, "postgresql://")
	if idx := strings.Index(dbURL, "@"); idx != -1 {
		dbURL = dbURL[idx+1:]
	}
	if idx := strings.Index(dbURL, "/"); idx != -1 {
		dbURL = dbURL[:idx]
	}
	_, portStr, _ := net.SplitHostPort(dbURL)
	if portStr != "" {
		if p, err := strconv.Atoi(portStr); err == nil {
			return p
		}
	}
	return 5432
}

func parsePostgresUser() string {
	if user := os.Getenv("SAMPMAIL_PG_USER"); user != "" {
		return user
	}
	dbURL := parseDatabaseURL()
	if dbURL == "" {
		return ""
	}
	// Parse: postgres://user:pass@host:port/db
	dbURL = strings.TrimPrefix(dbURL, "postgres://")
	dbURL = strings.TrimPrefix(dbURL, "postgresql://")
	if idx := strings.Index(dbURL, "@"); idx != -1 {
		userPass := dbURL[:idx]
		if colonIdx := strings.Index(userPass, ":"); colonIdx != -1 {
			return userPass[:colonIdx]
		}
		return userPass
	}
	return ""
}

func parsePostgresPassword() string {
	if pass := os.Getenv("SAMPMAIL_PG_PASSWORD"); pass != "" {
		return pass
	}
	dbURL := parseDatabaseURL()
	if dbURL == "" {
		return ""
	}
	// Parse: postgres://user:pass@host:port/db
	dbURL = strings.TrimPrefix(dbURL, "postgres://")
	dbURL = strings.TrimPrefix(dbURL, "postgresql://")
	if idx := strings.Index(dbURL, "@"); idx != -1 {
		userPass := dbURL[:idx]
		if colonIdx := strings.Index(userPass, ":"); colonIdx != -1 {
			return userPass[colonIdx+1:]
		}
	}
	return ""
}

func parsePostgresDatabase() string {
	if db := os.Getenv("SAMPMAIL_PG_DATABASE"); db != "" {
		return db
	}
	dbURL := parseDatabaseURL()
	if dbURL == "" {
		return ""
	}
	// Parse: postgres://user:pass@host:port/db?sslmode=disable
	dbURL = strings.TrimPrefix(dbURL, "postgres://")
	dbURL = strings.TrimPrefix(dbURL, "postgresql://")
	if idx := strings.Index(dbURL, "/"); idx != -1 {
		dbPart := dbURL[idx+1:]
		// Remove query params
		if qIdx := strings.Index(dbPart, "?"); qIdx != -1 {
			dbPart = dbPart[:qIdx]
		}
		return dbPart
	}
	return ""
}

func parsePostgresSSLMode() string {
	if mode := os.Getenv("SAMPMAIL_PG_SSLMODE"); mode != "" {
		return mode
	}
	dbURL := parseDatabaseURL()
	if dbURL == "" {
		return "require"
	}
	// Parse ?sslmode=disable from URL
	if idx := strings.Index(dbURL, "sslmode="); idx != -1 {
		sslPart := dbURL[idx+8:]
		if endIdx := strings.Index(sslPart, "&"); endIdx != -1 {
			return sslPart[:endIdx]
		}
		return sslPart
	}
	return "disable" // Default to disable for local dev
}

// deriveBaseURL builds the canonical public-facing base URL.
// Priority:
//  1. SAMPMAIL_BASE_URL env var (explicit, recommended for production)
//  2. https://${SAMPMAIL_LISTEN_ADDR} when TLS is enabled
//  3. http://${SAMPMAIL_LISTEN_ADDR} as last resort
//
// The value is used as the root for tracking links, unsubscribe URLs, and
// one-click RFC-8058 headers.  It is also auto-saved to AppSettings on
// first startup so the UI reflects the correct hostname without manual
// configuration.
func deriveBaseURL() string {
	if v := os.Getenv("SAMPMAIL_BASE_URL"); v != "" {
		// Strip any trailing slash so callers can always do baseURL+"/path"
		return strings.TrimRight(v, "/")
	}
	// Fall back to the listen address
	addr := getEnv("SAMPMAIL_LISTEN_ADDR", "0.0.0.0:9000")
	// Replace 0.0.0.0 / :: with localhost so URLs are at least usable locally
	if strings.HasPrefix(addr, "0.0.0.0:") {
		addr = "localhost" + addr[7:]
	} else if strings.HasPrefix(addr, "[::]:") {
		addr = "localhost" + addr[4:]
	}
	scheme := "http"
	if getEnvBool("SAMPMAIL_TLS_ENABLED", false) {
		scheme = "https"
	}
	return scheme + "://" + addr
}
