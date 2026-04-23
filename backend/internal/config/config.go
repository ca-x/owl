package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

type Config struct {
	Port                 string
	JWTSecret            string
	DataDir              string
	UploadsDir           string
	LibraryDir           string
	DatabasePath         string
	DatabaseDSN          string
	FrontendOrigin       string
	BootstrapAdmin       bool
	AllowRegister        bool
	AdminUsername        string
	AdminPassword        string
	RedisAddr            string
	RedisPassword        string
	RedisDB              int
	RedisKeyPrefix       string
	RedisPrefixMaxLen    int
	RedisSearchKeyPrefix string
	RedisSearchEnabled   bool
	AudioCacheDir        string
	FFmpegBin            string
}

func Load() (Config, error) {
	dataDir := getEnv("OWL_DATA_DIR", "./data")
	uploadsDir := getEnv("OWL_UPLOADS_DIR", filepath.Join(dataDir, "uploads"))
	databasePath := getEnv("OWL_DB_PATH", filepath.Join(dataDir, "data.db"))
	jwtSecret := strings.TrimSpace(getEnv("OWL_JWT_SECRET", "dev-secret-change-me"))
	if jwtSecret == "" {
		return Config{}, fmt.Errorf("OWL_JWT_SECRET is required")
	}
	cfg := Config{
		Port:                 getEnv("OWL_PORT", "8080"),
		JWTSecret:            jwtSecret,
		DataDir:              dataDir,
		UploadsDir:           uploadsDir,
		LibraryDir:           getEnv("OWL_LIBRARY_DIR", uploadsDir),
		DatabasePath:         databasePath,
		DatabaseDSN:          sqliteDSN(databasePath),
		FrontendOrigin:       getEnv("OWL_FRONTEND_ORIGIN", "*"),
		BootstrapAdmin:       getEnvBool("OWL_BOOTSTRAP_ADMIN", false),
		AllowRegister:        getEnvBool("OWL_ALLOW_REGISTER", true),
		AdminUsername:        strings.TrimSpace(getEnv("OWL_ADMIN_USERNAME", "admin")),
		AdminPassword:        getEnv("OWL_ADMIN_PASSWORD", "admin123456"),
		RedisAddr:            strings.TrimSpace(os.Getenv("OWL_REDIS_ADDR")),
		RedisPassword:        os.Getenv("OWL_REDIS_PASSWORD"),
		RedisDB:              getEnvInt("OWL_REDIS_DB", 0),
		RedisKeyPrefix:       getEnv("OWL_REDIS_KEY_PREFIX", "owl:mdx:index"),
		RedisPrefixMaxLen:    getEnvInt("OWL_REDIS_PREFIX_MAX_LEN", 8),
		RedisSearchKeyPrefix: getEnv("OWL_REDIS_SEARCH_KEY_PREFIX", "owl:mdx:search"),
		RedisSearchEnabled:   getEnvBool("OWL_REDIS_SEARCH_ENABLED", true),
		AudioCacheDir:        getEnv("OWL_AUDIO_CACHE_DIR", filepath.Join(dataDir, "cache", "audio")),
		FFmpegBin:            strings.TrimSpace(os.Getenv("FFMPEG_BIN")),
	}
	return cfg, nil
}

func sqliteDSN(path string) string {
	return fmt.Sprintf("file:%s?cache=shared&_pragma=foreign_keys(1)&_pragma=journal_mode(WAL)&_pragma=synchronous(NORMAL)&_pragma=busy_timeout(10000)", path)
}

func EnsureRuntimeDirs(cfg Config) error {
	for _, dir := range []string{cfg.DataDir, cfg.UploadsDir, cfg.LibraryDir} {
		if strings.TrimSpace(dir) == "" {
			continue
		}
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return fmt.Errorf("create runtime dir %s: %w", dir, err)
		}
	}
	return nil
}

func getEnv(key, fallback string) string {
	if value := os.Getenv(key); strings.TrimSpace(value) != "" {
		return value
	}
	return fallback
}

func getEnvBool(key string, fallback bool) bool {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	parsed, err := strconv.ParseBool(value)
	if err != nil {
		return fallback
	}
	return parsed
}

func getEnvInt(key string, fallback int) int {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return fallback
	}
	return parsed
}
