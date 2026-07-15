package bootstrap

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	ListenAddress         string
	SQLitePath            string
	AssistantMCPEndpoint  string
	AgentDriver           string
	AgentProfilePath      string
	AgentMaxIterations    int
	AgentMaxToolCalls     int
	AgentTimeout          time.Duration
	ModelTimeout          time.Duration
	MCPToolTimeout        time.Duration
	IncidentAlertSource   string
	IncidentTenantID      string
	IncidentOrgID         int
	IncidentActorID       string
	IncidentWebhookSecret string
	ApprovalEvidenceKey   string
	IncidentAlertMaxSkew  time.Duration
	DeepSeekAPIKey        string
	DeepSeekBaseURL       string
	DeepSeekModel         string
}

func LoadConfigFromEnvironment() (Config, error) {
	config := Config{
		ListenAddress:         env("AI_CORE_LISTEN_ADDRESS", ":8080"),
		SQLitePath:            env("AI_CORE_SQLITE_PATH", "data/ai-core.sqlite"),
		AssistantMCPEndpoint:  env("ASSISTANT_MCP_ENDPOINT", "http://127.0.0.1:8081/mcp"),
		AgentDriver:           env("AI_CORE_AGENT_DRIVER", "mock"),
		AgentProfilePath:      env("AI_CORE_AGENT_PROFILE_PATH", "data/agent-knowledge/node_exporter.md"),
		AgentMaxIterations:    envInt("AI_CORE_AGENT_MAX_ITERATIONS", 6),
		AgentMaxToolCalls:     envInt("AI_CORE_AGENT_MAX_TOOL_CALLS", 12),
		AgentTimeout:          envDuration("AI_CORE_AGENT_TIMEOUT", 60*time.Second),
		ModelTimeout:          envDuration("AI_CORE_MODEL_TIMEOUT", 30*time.Second),
		MCPToolTimeout:        envDuration("AI_CORE_MCP_TOOL_TIMEOUT", 12*time.Second),
		IncidentAlertSource:   env("AI_CORE_INCIDENT_ALERT_SOURCE", "demo-grafana"),
		IncidentTenantID:      env("AI_CORE_INCIDENT_TENANT_ID", "org:1"),
		IncidentOrgID:         envInt("AI_CORE_INCIDENT_ORG_ID", 1),
		IncidentActorID:       env("AI_CORE_INCIDENT_ACTOR_ID", "system:grafana"),
		IncidentWebhookSecret: strings.TrimSpace(os.Getenv("ORDER_INCIDENT_WEBHOOK_SECRET")),
		ApprovalEvidenceKey:   strings.TrimSpace(os.Getenv("ORDER_INCIDENT_APPROVAL_EVIDENCE_KEY")),
		IncidentAlertMaxSkew:  envDuration("AI_CORE_INCIDENT_ALERT_MAX_SKEW", 5*time.Minute),
		DeepSeekAPIKey:        strings.TrimSpace(os.Getenv("DEEPSEEK_API_KEY")),
		DeepSeekBaseURL:       env("DEEPSEEK_BASE_URL", "https://api.deepseek.com"),
		DeepSeekModel:         env("DEEPSEEK_MODEL", "deepseek-v4-flash"),
	}
	return config, config.Validate()
}

func (c Config) Validate() error {
	if c.AgentDriver != "mock" && c.AgentDriver != "eino" {
		return fmt.Errorf("AI_CORE_AGENT_DRIVER must be mock or eino")
	}
	if c.AgentMaxIterations <= 0 || c.AgentMaxToolCalls <= 0 || c.AgentTimeout <= 0 || c.ModelTimeout <= 0 || c.MCPToolTimeout <= 0 {
		return fmt.Errorf("AI Core Agent and MCP timeouts and limits must be positive")
	}
	if c.IncidentWebhookSecret != "" && (len(c.IncidentWebhookSecret) < 16 || len(c.ApprovalEvidenceKey) < 32 || c.IncidentAlertSource == "" || c.IncidentTenantID == "" || c.IncidentOrgID < 1 || c.IncidentActorID == "" || c.IncidentAlertMaxSkew <= 0) {
		return fmt.Errorf("Incident alert source, identity, HMAC/ApprovalEvidence secrets and replay window are invalid")
	}
	if c.AgentDriver == "eino" {
		if c.DeepSeekAPIKey == "" {
			return fmt.Errorf("DEEPSEEK_API_KEY is required when AI_CORE_AGENT_DRIVER=eino")
		}
		if c.AgentProfilePath == "" || c.DeepSeekBaseURL == "" || c.DeepSeekModel == "" {
			return fmt.Errorf("Eino Agent Profile and DeepSeek model configuration are required")
		}
	}
	return nil
}

func env(key, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		return value
	}
	return fallback
}

func envInt(key string, fallback int) int {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return 0
	}
	return parsed
}

func envDuration(key string, fallback time.Duration) time.Duration {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	parsed, err := time.ParseDuration(value)
	if err != nil {
		return 0
	}
	return parsed
}
