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
}

func LoadConfigFromEnvironment() (Config, error) {
	config := Config{
		ListenAddress:           valueOrDefault(os.Getenv("ASSISTANT_MCP_LISTEN_ADDRESS"), ":8081"),
		FixtureDir:              os.Getenv("ASSISTANT_MCP_FIXTURE_DIR"),
		SchemaDir:               os.Getenv("ASSISTANT_MCP_SCHEMA_DIR"),
		PrometheusDriver:        valueOrDefault(os.Getenv("ASSISTANT_MCP_PROMETHEUS_DRIVER"), "mock"),
		PrometheusURL:           valueOrDefault(os.Getenv("ASSISTANT_MCP_PROMETHEUS_URL"), "http://prometheus:9090"),
		PrometheusDatasourceUID: valueOrDefault(os.Getenv("ASSISTANT_MCP_PROMETHEUS_DATASOURCE_UID"), registry.DatasourceUID),
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
	var err error
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
	return config, nil
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
