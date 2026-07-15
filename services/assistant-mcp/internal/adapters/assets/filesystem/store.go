package filesystem

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/santhosh-tekuri/jsonschema/v6"
	"gopkg.in/yaml.v3"
	"mini-torchbearing.local/services/assistant-mcp/internal/ports/assets"
	"mini-torchbearing.local/services/assistant-mcp/internal/runtime"
)

const (
	knowledgeSchema = "knowledge.schema.json"
	skillSchema     = "skill.schema.json"
	playbookSchema  = "playbook.schema.json"
	mappingSchema   = "alert-mapping.schema.json"
)

var readCapabilities = map[string]struct{}{
	"order_service.get_runtime":         {},
	"order_service.get_queue_snapshot":  {},
	"order_service.get_worker_state":    {},
	"order_service.get_worker_policy":   {},
	"order_service.get_recent_outcomes": {},
	"order_service.get_operation":       {},
}

type Store struct {
	knowledge map[string]Asset
	skills    map[string]Asset
	skillCaps map[string]map[string]struct{}
	playbooks map[string]Playbook
	mappings  []alertMapping
}

var _ assets.Port = (*Store)(nil)

func New(assetDirectory, schemaDirectory string) (*Store, error) {
	if strings.TrimSpace(assetDirectory) == "" || strings.TrimSpace(schemaDirectory) == "" {
		return nil, fmt.Errorf("asset and schema directories are required")
	}
	validators, err := compileSchemas(schemaDirectory)
	if err != nil {
		return nil, err
	}
	store := &Store{knowledge: map[string]Asset{}, skills: map[string]Asset{}, skillCaps: map[string]map[string]struct{}{}, playbooks: map[string]Playbook{}}
	if err := store.loadMarkdownDirectory(filepath.Join(assetDirectory, "knowledge"), "knowledge", validators[knowledgeSchema], store.knowledge); err != nil {
		return nil, err
	}
	if err := store.loadMarkdownDirectory(filepath.Join(assetDirectory, "skills"), "skill", validators[skillSchema], store.skills); err != nil {
		return nil, err
	}
	if err := store.loadPlaybooks(filepath.Join(assetDirectory, "playbooks"), validators[playbookSchema]); err != nil {
		return nil, err
	}
	if err := store.loadMappings(filepath.Join(assetDirectory, "mappings"), validators[mappingSchema]); err != nil {
		return nil, err
	}
	if len(store.knowledge) == 0 || len(store.skills) == 0 || len(store.playbooks) == 0 || len(store.mappings) == 0 {
		return nil, fmt.Errorf("operational asset set is incomplete")
	}
	if err := store.validateReferences(); err != nil {
		return nil, err
	}
	return store, nil
}

func (s *Store) Knowledge(id, version string) (Asset, error) {
	return findAsset(s.knowledge, id, version, "Knowledge")
}

func (s *Store) Skill(id, version string) (Asset, error) {
	return findAsset(s.skills, id, version, "Skill")
}

func (s *Store) Playbook(id, version, digest string) (Playbook, error) {
	playbook, exists := s.playbooks[assetKey(id, version)]
	if !exists {
		return Playbook{}, runtime.NewError(runtime.ResourceNotFound, "Playbook was not found", false)
	}
	if digest != "" && digest != playbook.Digest {
		return Playbook{}, runtime.NewError(runtime.SchemaValidationFailed, "Playbook digest does not match the pinned asset", false)
	}
	return clonePlaybook(playbook), nil
}

func (s *Store) Resolve(alert Alert) (Resolution, error) {
	if strings.TrimSpace(alert.SourceID) == "" || strings.TrimSpace(alert.AlertName) == "" || len(alert.Labels) > 20 {
		return Resolution{}, runtime.NewError(runtime.SchemaValidationFailed, "alert mapping input is invalid", false)
	}
	matches := make([]alertMapping, 0, 1)
	for _, mapping := range s.mappings {
		if mapping.SourceID != alert.SourceID || mapping.AlertName != alert.AlertName || !labelsMatch(mapping.RequiredLabels, alert.Labels) {
			continue
		}
		matches = append(matches, mapping)
	}
	if len(matches) == 0 {
		return Resolution{}, runtime.NewError(runtime.ResourceNotFound, "no Playbook mapping matches the alert", false)
	}
	if len(matches) != 1 {
		return Resolution{}, runtime.NewError(runtime.SchemaValidationFailed, "alert matches multiple Playbook mappings", false)
	}
	mapping := matches[0]
	playbook := s.playbooks[assetKey(mapping.Playbook.ID, mapping.Playbook.Version)]
	return Resolution{MappingID: mapping.ID, MappingDigest: mapping.Digest, PlaybookID: playbook.ID, PlaybookVersion: playbook.Version, PlaybookDigest: playbook.Digest, ServiceRef: mapping.ServiceRef}, nil
}

func (s *Store) loadMarkdownDirectory(directory, expectedKind string, schema *jsonschema.Schema, destination map[string]Asset) error {
	files, err := filesWithExtension(directory, ".md")
	if err != nil {
		return err
	}
	for _, path := range files {
		contents, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		metadata, body, err := parseMarkdown(contents)
		if err != nil {
			return fmt.Errorf("%s: %w", path, err)
		}
		metadata["content"] = body
		if err := validateValue(schema, metadata); err != nil {
			return fmt.Errorf("%s: %w", path, err)
		}
		kind, _ := metadata["kind"].(string)
		id, _ := metadata["id"].(string)
		version, _ := metadata["version"].(string)
		if kind != expectedKind {
			return fmt.Errorf("%s: unexpected asset kind", path)
		}
		key := assetKey(id, version)
		if _, exists := destination[key]; exists {
			return fmt.Errorf("%s: duplicate asset ID and version", path)
		}
		destination[key] = Asset{Kind: kind, ID: id, Version: version, Digest: digest(contents), Content: body}
		if expectedKind == "skill" {
			capabilities, ok := metadata["allowedCapabilities"].([]any)
			if !ok {
				return fmt.Errorf("%s: Skill capabilities have an invalid representation", path)
			}
			s.skillCaps[key] = make(map[string]struct{}, len(capabilities))
			for _, value := range capabilities {
				capability, ok := value.(string)
				if !ok {
					return fmt.Errorf("%s: Skill capability is not a string", path)
				}
				s.skillCaps[key][capability] = struct{}{}
			}
		}
	}
	return nil
}

func (s *Store) loadPlaybooks(directory string, schema *jsonschema.Schema) error {
	files, err := filesWithExtension(directory, ".yaml")
	if err != nil {
		return err
	}
	for _, path := range files {
		contents, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		generic, err := yamlJSON(contents)
		if err != nil {
			return fmt.Errorf("%s: %w", path, err)
		}
		if err := schema.Validate(generic); err != nil {
			return fmt.Errorf("%s: %w", path, err)
		}
		var playbook Playbook
		if err := decodeStrictYAML(contents, &playbook); err != nil {
			return fmt.Errorf("%s: %w", path, err)
		}
		for _, capability := range playbook.AllowedCapabilities {
			if _, exists := readCapabilities[capability]; !exists {
				return fmt.Errorf("%s: capability %q is not registered read-only", path, capability)
			}
		}
		playbook.Digest = digest(contents)
		key := assetKey(playbook.ID, playbook.Version)
		if _, exists := s.playbooks[key]; exists {
			return fmt.Errorf("%s: duplicate Playbook ID and version", path)
		}
		s.playbooks[key] = playbook
	}
	return nil
}

func (s *Store) loadMappings(directory string, schema *jsonschema.Schema) error {
	files, err := filesWithExtension(directory, ".yaml")
	if err != nil {
		return err
	}
	seenIDs := map[string]struct{}{}
	for _, path := range files {
		contents, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		generic, err := yamlJSON(contents)
		if err != nil {
			return fmt.Errorf("%s: %w", path, err)
		}
		if err := schema.Validate(generic); err != nil {
			return fmt.Errorf("%s: %w", path, err)
		}
		var set mappingSet
		if err := decodeStrictYAML(contents, &set); err != nil {
			return fmt.Errorf("%s: %w", path, err)
		}
		fileDigest := digest(contents)
		for _, mapping := range set.Mappings {
			if _, exists := seenIDs[mapping.ID]; exists {
				return fmt.Errorf("%s: duplicate mapping ID %q", path, mapping.ID)
			}
			seenIDs[mapping.ID] = struct{}{}
			mapping.Digest = fileDigest
			s.mappings = append(s.mappings, mapping)
		}
	}
	return nil
}

func (s *Store) validateReferences() error {
	for _, playbook := range s.playbooks {
		if _, exists := s.knowledge[assetKey(playbook.Knowledge.ID, playbook.Knowledge.Version)]; !exists {
			return fmt.Errorf("Playbook %s@%s references missing Knowledge", playbook.ID, playbook.Version)
		}
		skillKey := assetKey(playbook.Skill.ID, playbook.Skill.Version)
		_, exists := s.skills[skillKey]
		if !exists {
			return fmt.Errorf("Playbook %s@%s references missing Skill", playbook.ID, playbook.Version)
		}
		for _, capability := range playbook.AllowedCapabilities {
			if _, allowed := s.skillCaps[skillKey][capability]; !allowed {
				return fmt.Errorf("Playbook %s@%s capability %q is not allowed by its Skill", playbook.ID, playbook.Version, capability)
			}
		}
	}
	for _, mapping := range s.mappings {
		playbook, exists := s.playbooks[assetKey(mapping.Playbook.ID, mapping.Playbook.Version)]
		if !exists || playbook.ServiceRef != mapping.ServiceRef {
			return fmt.Errorf("mapping %s references a missing or incompatible Playbook", mapping.ID)
		}
	}
	return nil
}

func compileSchemas(directory string) (map[string]*jsonschema.Schema, error) {
	compiler := jsonschema.NewCompiler()
	result := map[string]*jsonschema.Schema{}
	for _, name := range []string{knowledgeSchema, skillSchema, playbookSchema, mappingSchema} {
		path := filepath.Join(directory, name)
		schema, err := compiler.Compile(path)
		if err != nil {
			return nil, fmt.Errorf("compile asset schema %s: %w", name, err)
		}
		result[name] = schema
	}
	return result, nil
}

func validateValue(schema *jsonschema.Schema, value any) error {
	contents, err := json.Marshal(value)
	if err != nil {
		return err
	}
	instance, err := jsonschema.UnmarshalJSON(bytes.NewReader(contents))
	if err != nil {
		return err
	}
	return schema.Validate(instance)
}

func yamlJSON(contents []byte) (any, error) {
	var value any
	if err := yaml.Unmarshal(contents, &value); err != nil {
		return nil, err
	}
	return validateJSONRoundTrip(value)
}

func validateJSONRoundTrip(value any) (any, error) {
	contents, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}
	return jsonschema.UnmarshalJSON(bytes.NewReader(contents))
}

func parseMarkdown(contents []byte) (map[string]any, string, error) {
	normalized := strings.ReplaceAll(string(contents), "\r\n", "\n")
	if !strings.HasPrefix(normalized, "---\n") {
		return nil, "", fmt.Errorf("missing YAML frontmatter")
	}
	parts := strings.SplitN(strings.TrimPrefix(normalized, "---\n"), "\n---\n", 2)
	if len(parts) != 2 || strings.TrimSpace(parts[1]) == "" {
		return nil, "", fmt.Errorf("missing Markdown content")
	}
	var metadata map[string]any
	if err := yaml.Unmarshal([]byte(parts[0]), &metadata); err != nil {
		return nil, "", err
	}
	return metadata, strings.TrimSpace(parts[1]), nil
}

func decodeStrictYAML(contents []byte, destination any) error {
	decoder := yaml.NewDecoder(bytes.NewReader(contents))
	decoder.KnownFields(true)
	return decoder.Decode(destination)
}

func filesWithExtension(directory, extension string) ([]string, error) {
	entries, err := os.ReadDir(directory)
	if err != nil {
		return nil, err
	}
	files := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != extension {
			continue
		}
		files = append(files, filepath.Join(directory, entry.Name()))
	}
	sort.Strings(files)
	return files, nil
}

func findAsset(assets map[string]Asset, id, version, kind string) (Asset, error) {
	asset, exists := assets[assetKey(id, version)]
	if !exists {
		return Asset{}, runtime.NewError(runtime.ResourceNotFound, kind+" asset was not found", false)
	}
	return asset, nil
}

func labelsMatch(required, actual map[string]string) bool {
	for name, value := range required {
		if actual[name] != value {
			return false
		}
	}
	return true
}

func assetKey(id, version string) string { return id + "@" + version }

func digest(contents []byte) string {
	sum := sha256.Sum256(contents)
	return hex.EncodeToString(sum[:])
}

func clonePlaybook(value Playbook) Playbook {
	value.AllowedCapabilities = append([]string{}, value.AllowedCapabilities...)
	value.Steps = append([]string{}, value.Steps...)
	return value
}
