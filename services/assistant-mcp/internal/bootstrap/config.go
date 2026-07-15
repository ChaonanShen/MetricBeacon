package bootstrap

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"mini-torchbearing.local/services/assistant-mcp/internal/adapters/prometheus/registry"
)

type Config struct {
	ListenAddress           string
	FixtureDir              string
	SchemaDir               string
	PrometheusDriver        string
	PrometheusURL           string
	PrometheusDatasourceUID string
	PrometheusTimeout       time.Duration
	IncidentEnabled         bool
	RemediationEnabled      bool
	IncidentToolSchemaDir   string
	AssetDir                string
	AssetSchemaDir          string
	OrderDriver             string
	OrderURL                string
	OrderReadToken          string
	OrderRemediationToken   string
	OrderTimeout            time.Duration
	OrderMockScenario       string
	CheckpointKey           string
	ApprovalEvidenceKey     string
	ExecutionAuditPath      string
}

func LoadConfigFromEnvironment() (Config, error) {
	config := Config{
		ListenAddress:           valueOrDefault(os.Getenv("ASSISTANT_MCP_LISTEN_ADDRESS"), ":8081"),
		FixtureDir:              os.Getenv("ASSISTANT_MCP_FIXTURE_DIR"),
		SchemaDir:               os.Getenv("ASSISTANT_MCP_SCHEMA_DIR"),
		PrometheusDriver:        valueOrDefault(os.Getenv("ASSISTANT_MCP_PROMETHEUS_DRIVER"), "mock"),
		PrometheusURL:           valueOrDefault(os.Getenv("ASSISTANT_MCP_PROMETHEUS_URL"), "http://prometheus:9090"),
		PrometheusDatasourceUID: valueOrDefault(os.Getenv("ASSISTANT_MCP_PROMETHEUS_DATASOURCE_UID"), registry.DatasourceUID),
		IncidentToolSchemaDir:   os.Getenv("ASSISTANT_MCP_INCIDENT_SCHEMA_DIR"),
		AssetDir:                os.Getenv("ASSISTANT_MCP_ASSET_DIR"),
		AssetSchemaDir:          os.Getenv("ASSISTANT_MCP_ASSET_SCHEMA_DIR"),
		OrderDriver:             valueOrDefault(os.Getenv("ASSISTANT_MCP_ORDER_DRIVER"), "http"),
		OrderURL:                valueOrDefault(os.Getenv("ASSISTANT_MCP_ORDER_URL"), "http://order-service:8091"),
		OrderReadToken:          os.Getenv("ASSISTANT_MCP_ORDER_READ_TOKEN"),
		OrderRemediationToken:   os.Getenv("ASSISTANT_MCP_ORDER_REMEDIATION_TOKEN"),
		OrderMockScenario:       valueOrDefault(os.Getenv("ASSISTANT_MCP_ORDER_MOCK_SCENARIO"), "healthy"),
		CheckpointKey:           os.Getenv("ASSISTANT_MCP_CHECKPOINT_KEY"),
		ApprovalEvidenceKey:     os.Getenv("ORDER_INCIDENT_APPROVAL_EVIDENCE_KEY"),
		ExecutionAuditPath:      valueOrDefault(os.Getenv("ASSISTANT_MCP_EXECUTION_AUDIT_PATH"), "/var/lib/assistant-mcp/execution-audit.jsonl"),
	}
	var err error
	config.IncidentEnabled, err = parseBoolean("ASSISTANT_MCP_INCIDENT_ENABLED", os.Getenv("ASSISTANT_MCP_INCIDENT_ENABLED"))
	if err != nil {
		return Config{}, err
	}
	config.RemediationEnabled, err = parseBoolean("ASSISTANT_MCP_REMEDIATION_ENABLED", os.Getenv("ASSISTANT_MCP_REMEDIATION_ENABLED"))
	if err != nil {
		return Config{}, err
	}
	if config.FixtureDir == "" {
		var err error
		config.FixtureDir, err = findRepositoryPath("data/mock-scenarios/node_exporter_overview")
		if err != nil {
			return Config{}, err
		}
	}
	if config.SchemaDir == "" {
		var err error
		config.SchemaDir, err = findRepositoryPath("contracts/tools/grafana")
		if err != nil {
			return Config{}, err
		}
	}
	config.PrometheusTimeout, err = time.ParseDuration(valueOrDefault(os.Getenv("ASSISTANT_MCP_PROMETHEUS_TIMEOUT"), "10s"))
	if err != nil || config.PrometheusTimeout <= 0 {
		return Config{}, fmt.Errorf("ASSISTANT_MCP_PROMETHEUS_TIMEOUT must be a positive duration")
	}
	if config.PrometheusDriver != "mock" && config.PrometheusDriver != "http" {
		return Config{}, fmt.Errorf("ASSISTANT_MCP_PROMETHEUS_DRIVER must be mock or http")
	}
	if config.PrometheusDatasourceUID != registry.DatasourceUID {
		return Config{}, fmt.Errorf("ASSISTANT_MCP_PROMETHEUS_DATASOURCE_UID must be %s", registry.DatasourceUID)
	}
	config.OrderTimeout, err = time.ParseDuration(valueOrDefault(os.Getenv("ASSISTANT_MCP_ORDER_TIMEOUT"), "3s"))
	if err != nil || config.OrderTimeout <= 0 {
		return Config{}, fmt.Errorf("ASSISTANT_MCP_ORDER_TIMEOUT must be a positive duration")
	}
	if config.IncidentEnabled {
		if config.IncidentToolSchemaDir == "" {
			config.IncidentToolSchemaDir, err = findRepositoryPath("contracts/tools/incident")
			if err != nil {
				return Config{}, err
			}
		}
		if config.AssetDir == "" {
			config.AssetDir, err = findRepositoryPath("data/operational-assets")
			if err != nil {
				return Config{}, err
			}
		}
		if config.AssetSchemaDir == "" {
			config.AssetSchemaDir, err = findRepositoryPath("contracts/schemas/assets")
			if err != nil {
				return Config{}, err
			}
		}
		if config.OrderDriver != "mock" && config.OrderDriver != "http" {
			return Config{}, fmt.Errorf("ASSISTANT_MCP_ORDER_DRIVER must be mock or http")
		}
		if config.OrderDriver == "http" && strings.TrimSpace(config.OrderReadToken) == "" {
			return Config{}, fmt.Errorf("ASSISTANT_MCP_ORDER_READ_TOKEN is required for the HTTP order driver")
		}
		if len(config.CheckpointKey) < 32 {
			return Config{}, fmt.Errorf("ASSISTANT_MCP_CHECKPOINT_KEY must contain at least 32 bytes")
		}
		if config.RemediationEnabled {
			if len(config.ApprovalEvidenceKey) < 32 || strings.TrimSpace(config.ExecutionAuditPath) == "" {
				return Config{}, fmt.Errorf("ORDER_INCIDENT_APPROVAL_EVIDENCE_KEY and execution audit path are required for remediation")
			}
			if config.OrderDriver == "http" && strings.TrimSpace(config.OrderRemediationToken) == "" {
				return Config{}, fmt.Errorf("ASSISTANT_MCP_ORDER_REMEDIATION_TOKEN is required for HTTP remediation")
			}
		}
	} else if config.RemediationEnabled {
		return Config{}, fmt.Errorf("ASSISTANT_MCP_REMEDIATION_ENABLED requires the Incident profile")
	}
	return config, nil
}

func parseBoolean(name, value string) (bool, error) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "", "false", "0":
		return false, nil
	case "true", "1":
		return true, nil
	default:
		return false, fmt.Errorf("%s must be true or false", name)
	}
}

func findRepositoryPath(relative string) (string, error) {
	directory, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		candidate := filepath.Join(directory, relative)
		if info, err := os.Stat(candidate); err == nil && info.IsDir() {
			return candidate, nil
		}
		parent := filepath.Dir(directory)
		if parent == directory {
			break
		}
		directory = parent
	}
	return "", fmt.Errorf("could not locate required repository directory %q", relative)
}

func valueOrDefault(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}
