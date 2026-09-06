package scenario

import (
	"bytes"
	"fmt"

	"gopkg.in/yaml.v3"

	"gust/pkg/api"
)

// RenderYAML emits a human-reviewed scenario template with H8 warnings.
func RenderYAML(sc *api.TestScenario) ([]byte, error) {
	type yamlScenario struct {
		ID          string                    `yaml:"id"`
		Version     string                    `yaml:"version"`
		Description string                    `yaml:"description"`
		Task        api.TaskInfo              `yaml:"task"`
		Environment api.EnvironmentSpec       `yaml:"environment"`
		Assertions  []api.Assertion           `yaml:"assertions"`
		Reliability api.ReliabilityConfig     `yaml:"reliability"`
		Provenance  api.TestScenarioProvenance `yaml:"provenance"`
	}

	payload := yamlScenario{
		ID:          sc.ID,
		Version:     sc.Version,
		Description: sc.Description,
		Task:        sc.Task,
		Environment: sc.Environment,
		Assertions:  sc.Assertions,
		Reliability: sc.Reliability,
		Provenance:  sc.Provenance,
	}

	var body bytes.Buffer
	enc := yaml.NewEncoder(&body)
	enc.SetIndent(2)
	if err := enc.Encode(payload); err != nil {
		return nil, err
	}
	_ = enc.Close()

	warning := []byte(`# -----------------------------------------------------------------------------
# WARNING: Trace is not the test!
# Assertions are left blank by design. Declare what the agent SHOULD do here,
# not merely what happened in the original captured trace.
# -----------------------------------------------------------------------------
`)
	// Insert warning before assertions key if present.
	out := body.Bytes()
	marker := []byte("assertions:")
	idx := bytes.Index(out, marker)
	if idx >= 0 {
		var buf bytes.Buffer
		buf.Write(out[:idx])
		buf.Write(warning)
		buf.Write(out[idx:])
		return buf.Bytes(), nil
	}
	return append(warning, out...), nil
}

// ExtractYAML is ExtractFromRun + RenderYAML.
func (e *Extractor) ExtractYAML(run api.AgentRun) (*api.TestScenario, []byte, error) {
	sc, err := e.ExtractFromRun(run)
	if err != nil {
		return nil, nil, err
	}
	raw, err := RenderYAML(sc)
	if err != nil {
		return nil, nil, fmt.Errorf("render yaml: %w", err)
	}
	return sc, raw, nil
}
