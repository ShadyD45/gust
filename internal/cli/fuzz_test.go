package cli

import (
	"testing"

	"gopkg.in/yaml.v3"

	"gust/pkg/api"
)

func FuzzPolicyYAML(f *testing.F) {
	f.Add([]byte(`version: "1.0"
name: default
reliability:
  default_minimum_pass_rate: 0.95
  min_samples_for_verdict: 5
  on_flaky: warn
`))
	f.Fuzz(func(t *testing.T, data []byte) {
		var p api.Policy
		if err := yaml.Unmarshal(data, &p); err != nil {
			return
		}
		_ = p.Validate()
	})
}
