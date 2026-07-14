package profile_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"mini-torchbearing.local/services/ai-core/internal/adapters/outbound/agent/profile"
)

func TestLoadRepositoryProfileAndViews(t *testing.T) {
	loaded, err := profile.Load(repositoryPath(t, "data/agent-knowledge/node_exporter.md"))
	if err != nil || !strings.Contains(loaded.Content, "node_load1") {
		t.Fatalf("unexpected Profile: %#v, %v", loaded, err)
	}
	views := profile.Views()
	if len(views) != 3 || views[0].Key != "cpu" || views[2].CanonicalExpression != "node_load1" {
		t.Fatalf("unexpected view metadata: %#v", views)
	}
	if _, ok := profile.ViewForKey("unknown"); ok {
		t.Fatal("unknown view was accepted")
	}
}

func TestLoadRejectsMissingGuidanceInvalidUTF8AndOversize(t *testing.T) {
	directory := t.TempDir()
	for name, contents := range map[string][]byte{
		"missing.md": []byte("## 支持视图和规范 PromQL\nnode_load1"),
		"invalid.md": []byte{0xff},
		"large.md":   []byte(strings.Repeat("x", profile.MaxBytes+1)),
	} {
		path := filepath.Join(directory, name)
		if err := os.WriteFile(path, contents, 0o600); err != nil {
			t.Fatal(err)
		}
		if _, err := profile.Load(path); err == nil {
			t.Fatalf("invalid profile %s was accepted", name)
		}
	}
}

func repositoryPath(t *testing.T, relative string) string {
	t.Helper()
	directory, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for {
		candidate := filepath.Join(directory, relative)
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
		parent := filepath.Dir(directory)
		if parent == directory {
			t.Fatalf("repository path %q was not found", relative)
		}
		directory = parent
	}
}
