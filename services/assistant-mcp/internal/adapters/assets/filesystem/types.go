package filesystem

import "mini-torchbearing.local/services/assistant-mcp/internal/ports/assets"

type Asset = assets.Asset
type AssetRef = assets.AssetRef
type PreparePolicy = assets.PreparePolicy
type Playbook = assets.Playbook
type Alert = assets.Alert
type Resolution = assets.Resolution

type mappingSet struct {
	Kind     string         `yaml:"kind" json:"kind"`
	Version  string         `yaml:"version" json:"version"`
	Mappings []alertMapping `yaml:"mappings" json:"mappings"`
}

type alertMapping struct {
	ID             string            `yaml:"id" json:"id"`
	SourceID       string            `yaml:"sourceId" json:"sourceId"`
	AlertName      string            `yaml:"alertName" json:"alertName"`
	RequiredLabels map[string]string `yaml:"requiredLabels" json:"requiredLabels"`
	Playbook       AssetRef          `yaml:"playbook" json:"playbook"`
	ServiceRef     string            `yaml:"serviceRef" json:"serviceRef"`
	Digest         string            `yaml:"-" json:"-"`
}
