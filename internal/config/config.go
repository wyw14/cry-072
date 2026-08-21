package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	Environment        string
	HTTPAddress        string
	DatabaseURL        string
	DatabaseTimeout    time.Duration
	ShutdownTimeout    time.Duration
	AttachmentDir      string
	AttachmentMaxBytes int64
	DemoOperatorID     string
	DemoOperatorRole   string
}

func Load() (Config, error) {
	return LoadFrom(os.LookupEnv)
}

func LoadFrom(lookup func(string) (string, bool)) (Config, error) {
	cfg := Config{
		Environment:        readString(lookup, "APP_ENV", "development"),
		HTTPAddress:        readString(lookup, "HTTP_ADDR", ":8080"),
		DatabaseURL:        readString(lookup, "DATABASE_URL", ""),
		DatabaseTimeout:    5 * time.Second,
		ShutdownTimeout:    10 * time.Second,
		AttachmentDir:      readString(lookup, "ATTACHMENT_DIR", "./runtime/attachments"),
		AttachmentMaxBytes: 8 << 20,
		DemoOperatorID:     readString(lookup, "DEMO_OPERATOR_ID", "operator-demo"),
		DemoOperatorRole:   readString(lookup, "DEMO_OPERATOR_ROLE", "supervisor"),
	}

	var err error
	if cfg.DatabaseTimeout, err = readDuration(lookup, "DATABASE_TIMEOUT", cfg.DatabaseTimeout); err != nil {
		return Config{}, err
	}
	if cfg.ShutdownTimeout, err = readDuration(lookup, "SHUTDOWN_TIMEOUT", cfg.ShutdownTimeout); err != nil {
		return Config{}, err
	}
	if cfg.AttachmentMaxBytes, err = readInt64(lookup, "ATTACHMENT_MAX_BYTES", cfg.AttachmentMaxBytes); err != nil {
		return Config{}, err
	}
	return cfg, cfg.validate()
}

func (cfg Config) validate() error {
	if cfg.AttachmentMaxBytes < 1024 || cfg.AttachmentMaxBytes > 64<<20 {
		return fmt.Errorf("ATTACHMENT_MAX_BYTES must be between 1024 and 67108864")
	}
	if cfg.DatabaseTimeout <= 0 || cfg.ShutdownTimeout <= 0 {
		return fmt.Errorf("timeouts must be positive")
	}
	if !strings.HasPrefix(cfg.HTTPAddress, ":") && !strings.Contains(cfg.HTTPAddress, ":") {
		return fmt.Errorf("HTTP_ADDR must include a port")
	}
	return nil
}

func readString(lookup func(string) (string, bool), key, fallback string) string {
	value, exists := lookup(key)
	if value = strings.TrimSpace(value); exists && value != "" {
		return value
	}
	return fallback
}

func readDuration(lookup func(string) (string, bool), key string, fallback time.Duration) (time.Duration, error) {
	value := readString(lookup, key, "")
	if value == "" {
		return fallback, nil
	}
	parsed, err := time.ParseDuration(value)
	if err != nil {
		return 0, fmt.Errorf("%s: %w", key, err)
	}
	return parsed, nil
}

func readInt64(lookup func(string) (string, bool), key string, fallback int64) (int64, error) {
	value := readString(lookup, key, "")
	if value == "" {
		return fallback, nil
	}
	parsed, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("%s: %w", key, err)
	}
	return parsed, nil
}
