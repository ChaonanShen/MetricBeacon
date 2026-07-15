package filesystem_test

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"mini-torchbearing.local/services/assistant-mcp/internal/adapters/assets/filesystem"
	"mini-torchbearing.local/services/assistant-mcp/internal/runtime"
)

func TestStoreLoadsPinsAndResolvesAssets(t *testing.T) {
	store := loadRepositoryStore(t)
	knowledge, err := store.Knowledge("order-service", "1")
	if err != nil || knowledge.Kind != "knowledge" || len(knowledge.Digest) != 64 || !strings.Contains(knowledge.Content, "Backlog is a symptom") {
		t.Fatalf("unexpected Knowledge: %#v %v", knowledge, err)
	}
	skill, err := store.Skill("diagnose-order-backlog", "1")
	if err != nil || skill.Kind != "skill" || len(skill.Digest) != 64 || !strings.Contains(skill.Content, "insufficient evidence") {
		t.Fatalf("unexpected Skill: %#v %v", skill, err)
	}
	resolution, err := store.Resolve(filesystem.Alert{SourceID: "demo-grafana", AlertName: "OrderQueueBacklog", Labels: map[string]string{"service_ref": "order-demo", "severity": "warning"}})
	if err != nil || resolution.MappingID != "demo-order-queue-backlog" || resolution.PlaybookID != "order-queue-backlog" || resolution.PlaybookVersion != "1" || len(resolution.MappingDigest) != 64 || len(resolution.PlaybookDigest) != 64 {
		t.Fatalf("unexpected resolution: %#v %v", resolution, err)
	}
	playbook, err := store.Playbook(resolution.PlaybookID, resolution.PlaybookVersion, resolution.PlaybookDigest)
	if err != nil || playbook.ServiceRef != "order-demo" || len(playbook.Steps) != 9 || playbook.PreparePolicy.HealthyConcurrency != 2 {
		t.Fatalf("unexpected Playbook: %#v %v", playbook, err)
	}
	playbook.Steps[0] = "mutated"
	again, _ := store.Playbook(resolution.PlaybookID, resolution.PlaybookVersion, resolution.PlaybookDigest)
	if again.Steps[0] != "load_assets" {
		t.Fatal("caller mutated stored Playbook")
	}
}

func TestStoreFailsClosedForUnknownAssetsMappingsAndDigest(t *testing.T) {
	store := loadRepositoryStore(t)
	_, err := store.Knowledge("missing", "1")
	requireCode(t, err, runtime.ResourceNotFound)
	_, err = store.Skill("diagnose-order-backlog", "2")
	requireCode(t, err, runtime.ResourceNotFound)
	_, err = store.Resolve(filesystem.Alert{SourceID: "demo-grafana", AlertName: "OtherAlert", Labels: map[string]string{"service_ref": "order-demo"}})
	requireCode(t, err, runtime.ResourceNotFound)
	_, err = store.Resolve(filesystem.Alert{SourceID: "demo-grafana", AlertName: "OrderQueueBacklog", Labels: map[string]string{"service_ref": "other"}})
	requireCode(t, err, runtime.ResourceNotFound)
	_, err = store.Playbook("order-queue-backlog", "1", strings.Repeat("0", 64))
	requireCode(t, err, runtime.SchemaValidationFailed)
	_, err = store.Resolve(filesystem.Alert{SourceID: "", AlertName: "OrderQueueBacklog", Labels: map[string]string{}})
	requireCode(t, err, runtime.SchemaValidationFailed)
}

func TestStoreRejectsUnknownCapabilityAndFreeFormStep(t *testing.T) {
	for _, test := range []struct {
		name string
		old  string
		new  string
	}{
		{"fault capability", "order_service.get_operation", "fault.inject"},
		{"write capability", "order_service.get_operation", "order_service.restore_worker_concurrency"},
		{"free-form step", "  - verify_business", "  - run_shell"},
	} {
		t.Run(test.name, func(t *testing.T) {
			assets, schemas := copiedFixture(t)
			mutate(t, filepath.Join(assets, "playbooks", "order-queue-backlog.v1.yaml"), test.old, test.new)
			if _, err := filesystem.New(assets, schemas); err == nil {
				t.Fatal("unsafe Playbook was accepted")
			}
		})
	}
}

func TestStoreRejectsInvalidOrBrokenAssetSets(t *testing.T) {
	tests := []struct {
		name, relative, old, replacement string
	}{
		{"fault marked healthy", "knowledge/order-service.v1.md", "workerConcurrency: 2", "workerConcurrency: 0"},
		{"missing knowledge reference", "playbooks/order-queue-backlog.v1.yaml", "id: order-service", "id: missing-knowledge"},
		{"unknown frontmatter", "knowledge/order-service.v1.md", "title: Order service facts", "unexpected: true\ntitle: Order service facts"},
		{"service mismatch", "mappings/order-demo.v1.yaml", "serviceRef: order-demo", "serviceRef: other-service"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			assets, schemas := copiedFixture(t)
			mutate(t, filepath.Join(assets, test.relative), test.old, test.replacement)
			if _, err := filesystem.New(assets, schemas); err == nil {
				t.Fatal("invalid asset set was accepted")
			}
		})
	}
}

func TestStoreRejectsAmbiguousAlertMapping(t *testing.T) {
	assets, schemas := copiedFixture(t)
	original := filepath.Join(assets, "mappings", "order-demo.v1.yaml")
	contents, err := os.ReadFile(original)
	if err != nil {
		t.Fatal(err)
	}
	second := strings.Replace(string(contents), "demo-order-queue-backlog", "demo-order-queue-backlog-copy", 1)
	if err := os.WriteFile(filepath.Join(assets, "mappings", "duplicate.v1.yaml"), []byte(second), 0o600); err != nil {
		t.Fatal(err)
	}
	store, err := filesystem.New(assets, schemas)
	if err != nil {
		t.Fatal(err)
	}
	_, err = store.Resolve(filesystem.Alert{SourceID: "demo-grafana", AlertName: "OrderQueueBacklog", Labels: map[string]string{"service_ref": "order-demo"}})
	requireCode(t, err, runtime.SchemaValidationFailed)
}

func TestDigestChangesWithAssetBytes(t *testing.T) {
	assets, schemas := copiedFixture(t)
	before, err := filesystem.New(assets, schemas)
	if err != nil {
		t.Fatal(err)
	}
	first, _ := before.Knowledge("order-service", "1")
	mutate(t, filepath.Join(assets, "knowledge", "order-service.v1.md"), "Backlog is a symptom", "A backlog is a symptom")
	after, err := filesystem.New(assets, schemas)
	if err != nil {
		t.Fatal(err)
	}
	second, _ := after.Knowledge("order-service", "1")
	if first.Digest == second.Digest {
		t.Fatal("content change did not change pinned digest")
	}
}

func loadRepositoryStore(t *testing.T) *filesystem.Store {
	t.Helper()
	root := repositoryRoot(t)
	store, err := filesystem.New(filepath.Join(root, "data", "operational-assets"), filepath.Join(root, "contracts", "schemas", "assets"))
	if err != nil {
		t.Fatal(err)
	}
	return store
}

func copiedFixture(t *testing.T) (string, string) {
	t.Helper()
	root := repositoryRoot(t)
	temporary := t.TempDir()
	assets := filepath.Join(temporary, "assets")
	schemas := filepath.Join(temporary, "schemas")
	copyDirectory(t, filepath.Join(root, "data", "operational-assets"), assets)
	copyDirectory(t, filepath.Join(root, "contracts", "schemas", "assets"), schemas)
	return assets, schemas
}

func copyDirectory(t *testing.T, source, destination string) {
	t.Helper()
	if err := filepath.WalkDir(source, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		relative, _ := filepath.Rel(source, path)
		target := filepath.Join(destination, relative)
		if entry.IsDir() {
			return os.MkdirAll(target, 0o700)
		}
		contents, readErr := os.ReadFile(path)
		if readErr != nil {
			return readErr
		}
		return os.WriteFile(target, contents, 0o600)
	}); err != nil {
		t.Fatal(err)
	}
}

func mutate(t *testing.T, path, old, replacement string) {
	t.Helper()
	contents, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	updated := strings.Replace(string(contents), old, replacement, 1)
	if updated == string(contents) {
		t.Fatalf("mutation target %q not found in %s", old, path)
	}
	if err := os.WriteFile(path, []byte(updated), 0o600); err != nil {
		t.Fatal(err)
	}
}

func repositoryRoot(t *testing.T) string {
	t.Helper()
	directory, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for {
		if _, err := os.Stat(filepath.Join(directory, "go.work")); err == nil {
			return directory
		}
		parent := filepath.Dir(directory)
		if parent == directory {
			t.Fatal("repository root not found")
		}
		directory = parent
	}
}

func requireCode(t *testing.T, err error, want runtime.ErrorCode) {
	t.Helper()
	var toolError *runtime.ToolError
	if !errors.As(err, &toolError) || toolError.Code != want {
		t.Fatalf("expected %s, got %v", want, err)
	}
}
