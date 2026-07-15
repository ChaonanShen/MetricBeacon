package remediation

import (
	"strings"
	"time"
)

type Intent struct {
	ID                string    `json:"id"`
	Digest            string    `json:"digest"`
	CapabilityID      string    `json:"capabilityId"`
	ServiceRef        string    `json:"serviceRef"`
	InstanceEpoch     string    `json:"instanceEpoch"`
	ExpectedVersion   int64     `json:"expectedVersion"`
	BeforeConcurrency int       `json:"beforeConcurrency"`
	AfterConcurrency  int       `json:"afterConcurrency"`
	Risk              string    `json:"risk"`
	CreatedAt         time.Time `json:"createdAt"`
}

func (i Intent) Valid(serviceRef string) bool {
	return i.ID != "" && ValidDigest(i.Digest) && i.CapabilityID == "order_service.restore_worker_concurrency" && i.ServiceRef == serviceRef && i.InstanceEpoch != "" && i.ExpectedVersion >= 1 && i.BeforeConcurrency == 0 && i.AfterConcurrency == 2 && i.Risk == "low" && !i.CreatedAt.IsZero()
}

func ValidDigest(value string) bool {
	if len(value) != len("sha256:")+64 || !strings.HasPrefix(value, "sha256:") {
		return false
	}
	for _, char := range value[len("sha256:"):] {
		if (char < '0' || char > '9') && (char < 'a' || char > 'f') {
			return false
		}
	}
	return true
}
