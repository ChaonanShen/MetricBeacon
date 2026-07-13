package bootstrap

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type Config struct {
	ListenAddress string
	FixtureDir    string
	SchemaDir     string
}

func LoadConfigFromEnvironment() (Config, error) {
	config := Config{
		ListenAddress: valueOrDefault(os.Getenv("ASSISTANT_MCP_LISTEN_ADDRESS"), ":8081"),
		FixtureDir:    os.Getenv("ASSISTANT_MCP_FIXTURE_DIR"),
		SchemaDir:     os.Getenv("ASSISTANT_MCP_SCHEMA_DIR"),
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
