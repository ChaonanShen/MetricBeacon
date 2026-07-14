package config

import (
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	Timeout     time.Duration
	MaxResponse int64
}

func Load() Config {
	return Config{Timeout: duration("AI_CORE_TIMEOUT", 10*time.Second), MaxResponse: size("AI_CORE_MAX_RESPONSE_BYTES", 4<<20)}
}
func env(key, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		return value
	}
	return fallback
}
func duration(key string, fallback time.Duration) time.Duration {
	value, err := time.ParseDuration(env(key, fallback.String()))
	if err != nil || value <= 0 {
		return fallback
	}
	return value
}
func size(key string, fallback int64) int64 {
	value, err := strconv.ParseInt(env(key, strconv.FormatInt(fallback, 10)), 10, 64)
	if err != nil || value <= 0 {
		return fallback
	}
	return value
}
