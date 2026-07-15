package bootstrap

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestMockConfigDoesNotRequireDeepSeekCredentials(t *testing.T) {
	config := Config{
		ListenAddress:        ":8080",
		SQLitePath:           filepath.Join(t.TempDir(), "ai-core.sqlite"),
		AssistantMCPEndpoint: "http://127.0.0.1:8081/mcp",
		AgentDriver:          "mock",
		AgentMaxIterations:   6,
		AgentMaxToolCalls:    12,
		AgentTimeout:         time.Minute,
		ModelTimeout:         30 * time.Second,
		MCPToolTimeout:       12 * time.Second,
	}
	application, err := New(context.Background(), config)
	if err != nil {
		t.Fatalf("Mock bootstrap unexpectedly required DeepSeek configuration: %v", err)
	}
	if err := application.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestEinoConfigRequiresKeyAndProfile(t *testing.T) {
	config := validEinoConfig(t)
	config.DeepSeekAPIKey = ""
	if err := config.Validate(); err == nil {
		t.Fatal("Eino config without API key was accepted")
	}

	config = validEinoConfig(t)
	config.AgentProfilePath = filepath.Join(t.TempDir(), "missing.md")
	if _, err := New(context.Background(), config); err == nil {
		t.Fatal("Eino bootstrap with a missing Profile was accepted")
	}
}

func TestConfigRejectsInvalidDriverAndLimits(t *testing.T) {
	config := validEinoConfig(t)
	config.AgentDriver = "other"
	if err := config.Validate(); err == nil {
		t.Fatal("unknown driver was accepted")
	}
	config = validEinoConfig(t)
	config.AgentMaxToolCalls = 0
	if err := config.Validate(); err == nil {
		t.Fatal("zero tool call limit was accepted")
	}
}

func TestLoadConfigFromEnvironmentUsesMockWithoutKey(t *testing.T) {
	t.Setenv("AI_CORE_AGENT_DRIVER", "mock")
	t.Setenv("DEEPSEEK_API_KEY", "")
	config, err := LoadConfigFromEnvironment()
	if err != nil {
		t.Fatal(err)
	}
	if config.AgentDriver != "mock" || config.DeepSeekAPIKey != "" || config.MCPToolTimeout != 12*time.Second || config.IncidentWebhookSecret != "" || config.ApprovalEvidenceKey != "" || config.IncidentAlertSource != "demo-grafana" || config.IncidentOrgID != 1 || config.IncidentAlertMaxSkew != 5*time.Minute {
		t.Fatalf("unexpected default config: %#v", config)
	}
}

func TestIncidentIngressConfigurationIsOptionalButClosedWhenEnabled(t *testing.T) {
	config := validEinoConfig(t)
	config.AgentDriver = "mock"
	config.DeepSeekAPIKey = ""
	config.IncidentWebhookSecret = "too-short"
	if err := config.Validate(); err == nil {
		t.Fatal("short Incident HMAC secret was accepted")
	}
	config.IncidentWebhookSecret = "0123456789abcdef"
	config.ApprovalEvidenceKey = "too-short"
	config.IncidentAlertSource = "demo-grafana"
	config.IncidentTenantID = "org:1"
	config.IncidentOrgID = 1
	config.IncidentActorID = "system:grafana"
	config.IncidentAlertMaxSkew = 5 * time.Minute
	if err := config.Validate(); err == nil {
		t.Fatal("short ApprovalEvidence key was accepted")
	}
	config.ApprovalEvidenceKey = "0123456789abcdef0123456789abcdef"
	if err := config.Validate(); err != nil {
		t.Fatal(err)
	}
	config.IncidentAlertMaxSkew = 0
	if err := config.Validate(); err == nil {
		t.Fatal("zero Incident replay window was accepted")
	}
}

func TestIncidentBootstrapAtomicallyEnablesApprovalExecutionService(t *testing.T) {
	config := validEinoConfig(t)
	config.AgentDriver = "mock"
	config.DeepSeekAPIKey = ""
	config.IncidentWebhookSecret = "0123456789abcdef"
	config.ApprovalEvidenceKey = "0123456789abcdef0123456789abcdef"
	config.IncidentAlertSource = "demo-grafana"
	config.IncidentTenantID = "org:1"
	config.IncidentOrgID = 1
	config.IncidentActorID = "system:grafana"
	config.IncidentAlertMaxSkew = 5 * time.Minute
	application, err := New(context.Background(), config)
	if err != nil {
		t.Fatal(err)
	}
	defer application.Close()
	if application.Handler.Approvals == nil || application.Handler.Incidents == nil || application.Handler.AlertIngress.HMACSecret == "" {
		t.Fatalf("Incident runtime was only partially wired: %#v", application.Handler)
	}
}

func validEinoConfig(t *testing.T) Config {
	t.Helper()
	return Config{
		ListenAddress:        ":8080",
		SQLitePath:           filepath.Join(t.TempDir(), "ai-core.sqlite"),
		AssistantMCPEndpoint: "http://127.0.0.1:8081/mcp",
		AgentDriver:          "eino",
		AgentProfilePath:     filepath.Join(repositoryRoot(t), "data/agent-knowledge/node_exporter.md"),
		AgentMaxIterations:   6,
		AgentMaxToolCalls:    12,
		AgentTimeout:         time.Minute,
		ModelTimeout:         30 * time.Second,
		MCPToolTimeout:       12 * time.Second,
		DeepSeekAPIKey:       "test-key",
		DeepSeekBaseURL:      "https://api.deepseek.com",
		DeepSeekModel:        "deepseek-v4-flash",
	}
}

func repositoryRoot(t *testing.T) string {
	t.Helper()
	directory, err := filepath.Abs(".")
	if err != nil {
		t.Fatal(err)
	}
	for {
		candidate := filepath.Join(directory, "data/agent-knowledge/node_exporter.md")
		if _, err := os.Stat(candidate); err == nil {
			return directory
		}
		parent := filepath.Dir(directory)
		if parent == directory {
			t.Fatal("repository root was not found")
		}
		directory = parent
	}
}
