package filesystem

type Asset struct {
	Kind    string
	ID      string
	Version string
	Digest  string
	Content string
}

type AssetRef struct {
	ID      string `yaml:"id" json:"id"`
	Version string `yaml:"version" json:"version"`
}

type PreparePolicy struct {
	CandidateAction          string `yaml:"candidateAction" json:"candidateAction"`
	ConfiguredConcurrency    int    `yaml:"configuredConcurrency" json:"configuredConcurrency"`
	EffectiveConcurrency     int    `yaml:"effectiveConcurrency" json:"effectiveConcurrency"`
	ActiveWorkers            int    `yaml:"activeWorkers" json:"activeWorkers"`
	HealthyConcurrency       int    `yaml:"healthyConcurrency" json:"healthyConcurrency"`
	MaxObservationAgeSeconds int    `yaml:"maxObservationAgeSeconds" json:"maxObservationAgeSeconds"`
}

type Playbook struct {
	Kind                string        `yaml:"kind" json:"kind"`
	ID                  string        `yaml:"id" json:"id"`
	Version             string        `yaml:"version" json:"version"`
	ServiceRef          string        `yaml:"serviceRef" json:"serviceRef"`
	Knowledge           AssetRef      `yaml:"knowledge" json:"knowledge"`
	Skill               AssetRef      `yaml:"skill" json:"skill"`
	AllowedCapabilities []string      `yaml:"allowedCapabilities" json:"allowedCapabilities"`
	Steps               []string      `yaml:"steps" json:"steps"`
	PreparePolicy       PreparePolicy `yaml:"preparePolicy" json:"preparePolicy"`
	Digest              string        `yaml:"-" json:"-"`
}

type Alert struct {
	SourceID  string
	AlertName string
	Labels    map[string]string
}

type Resolution struct {
	MappingID       string
	MappingDigest   string
	PlaybookID      string
	PlaybookVersion string
	PlaybookDigest  string
	ServiceRef      string
}

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
