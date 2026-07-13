package bootstrap

import (
	"os"
	"strings"
)

type Config struct {
	ListenAddress        string
	SQLitePath           string
	AssistantMCPEndpoint string
}

func LoadConfigFromEnvironment() Config {
	return Config{
		ListenAddress:        env("AI_CORE_LISTEN_ADDRESS", ":8080"),
		SQLitePath:           env("AI_CORE_SQLITE_PATH", "data/ai-core.sqlite"),
		AssistantMCPEndpoint: env("ASSISTANT_MCP_ENDPOINT", "http://127.0.0.1:8081/mcp"),
	}
}

func env(key, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		return value
	}
	return fallback
}
