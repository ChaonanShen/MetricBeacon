// Package profile loads the bounded, read-only node_exporter Agent Profile.
// It is owned by the outbound Agent adapter: Domain and Application never see
// its filesystem representation or view-specific rendering metadata.
package profile

import (
	"fmt"
	"os"
	"strings"
	"unicode/utf8"
)

const MaxBytes = 64 << 10

type Profile struct{ Content string }

type View struct {
	Key                 string
	Title               string
	Unit                string
	RefID               string
	CanonicalExpression string
}

var views = []View{
	{Key: "cpu", Title: "CPU 使用率", Unit: "percent", RefID: "A", CanonicalExpression: `100 * (1 - avg by (instance) (rate(node_cpu_seconds_total{mode="idle"}[5m])))`},
	{Key: "memory", Title: "内存可用率", Unit: "percent", RefID: "B", CanonicalExpression: `100 * node_memory_MemAvailable_bytes / node_memory_MemTotal_bytes`},
	{Key: "load", Title: "系统负载", Unit: "short", RefID: "C", CanonicalExpression: `node_load1`},
}

func Load(path string) (Profile, error) {
	contents, err := os.ReadFile(path)
	if err != nil {
		return Profile{}, fmt.Errorf("Agent Profile is unavailable")
	}
	if len(contents) == 0 || len(contents) > MaxBytes || !utf8.Valid(contents) {
		return Profile{}, fmt.Errorf("Agent Profile is invalid")
	}
	profile := Profile{Content: string(contents)}
	if err := profile.Validate(); err != nil {
		return Profile{}, err
	}
	return profile, nil
}

func (p Profile) Validate() error {
	for _, required := range []string{
		"## 支持视图和规范 PromQL", "## 指标解释口径", "## 无数据和错误处理", "## 最终回复格式", "## 禁止项",
		"node_cpu_seconds_total", "node_memory_MemAvailable_bytes", "node_memory_MemTotal_bytes", "node_load1", "private reasoning",
	} {
		if !strings.Contains(p.Content, required) {
			return fmt.Errorf("Agent Profile is missing required guidance")
		}
	}
	for _, view := range views {
		if !strings.Contains(p.Content, view.CanonicalExpression) {
			return fmt.Errorf("Agent Profile is missing a canonical expression")
		}
	}
	return nil
}

func Views() []View {
	result := make([]View, len(views))
	copy(result, views)
	return result
}

func ViewForKey(key string) (View, bool) {
	for _, view := range views {
		if view.Key == key {
			return view, true
		}
	}
	return View{}, false
}
