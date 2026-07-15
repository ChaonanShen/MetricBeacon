package bootstrap_test

import (
	"strings"
	"testing"

	"mini-torchbearing.local/services/assistant-mcp/internal/bootstrap"
)

func TestIncidentEnvironmentConfigurationFailsClosed(t *testing.T) {
	t.Setenv("ASSISTANT_MCP_INCIDENT_ENABLED", "true")
	t.Setenv("ASSISTANT_MCP_INCIDENT_SCHEMA_DIR", repositoryPath(t, "contracts/tools/incident"))
	t.Setenv("ASSISTANT_MCP_ASSET_DIR", repositoryPath(t, "data/operational-assets"))
	t.Setenv("ASSISTANT_MCP_ASSET_SCHEMA_DIR", repositoryPath(t, "contracts/schemas/assets"))
	t.Setenv("ASSISTANT_MCP_ORDER_DRIVER", "mock")
	t.Setenv("ASSISTANT_MCP_ORDER_MOCK_SCENARIO", "worker-stopped")
	t.Setenv("ASSISTANT_MCP_CHECKPOINT_KEY", "0123456789abcdef0123456789abcdef")
	config, err := bootstrap.LoadConfigFromEnvironment()
	if err != nil || !config.IncidentEnabled || config.OrderDriver != "mock" || config.OrderMockScenario != "worker-stopped" {
		t.Fatalf("unexpected incident config: %#v %v", config, err)
	}

	t.Setenv("ASSISTANT_MCP_CHECKPOINT_KEY", "short")
	if _, err := bootstrap.LoadConfigFromEnvironment(); err == nil || !strings.Contains(err.Error(), "32 bytes") {
		t.Fatalf("short checkpoint key was not rejected: %v", err)
	}
	t.Setenv("ASSISTANT_MCP_CHECKPOINT_KEY", "0123456789abcdef0123456789abcdef")
	t.Setenv("ASSISTANT_MCP_ORDER_DRIVER", "http")
	t.Setenv("ASSISTANT_MCP_ORDER_READ_TOKEN", "")
	if _, err := bootstrap.LoadConfigFromEnvironment(); err == nil || !strings.Contains(err.Error(), "READ_TOKEN") {
		t.Fatalf("missing order read token was not rejected: %v", err)
	}
	t.Setenv("ASSISTANT_MCP_INCIDENT_ENABLED", "sometimes")
	if _, err := bootstrap.LoadConfigFromEnvironment(); err == nil {
		t.Fatal("invalid incident boolean was accepted")
	}
}

func TestDisabledIncidentProfileRequiresNoIncidentSecrets(t *testing.T) {
	t.Setenv("ASSISTANT_MCP_INCIDENT_ENABLED", "false")
	t.Setenv("ASSISTANT_MCP_CHECKPOINT_KEY", "")
	t.Setenv("ASSISTANT_MCP_ORDER_READ_TOKEN", "")
	config, err := bootstrap.LoadConfigFromEnvironment()
	if err != nil || config.IncidentEnabled {
		t.Fatalf("disabled profile unexpectedly required incident configuration: %#v %v", config, err)
	}
}
