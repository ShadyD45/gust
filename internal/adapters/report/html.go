package report

import (
	"bytes"
	_ "embed"
	"fmt"
	"html/template"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gust/internal/core/policy"
	"gust/pkg/api"
)

const maxEvidenceRunes = 4000

//go:embed report.html.tmpl
var reportTemplateSource string

var reportTmpl = template.Must(template.New("report").Parse(reportTemplateSource))

// SuiteReport is the HTML-facing aggregate for one gust test invocation.
type SuiteReport struct {
	EvaluationID string
	GeneratedAt  time.Time
	PolicyName   string
	Overall      policy.Verdict
	Results      []*api.ReliabilityResult
}

type htmlView struct {
	GeneratedAt  string
	EvaluationID string
	PolicyName   string
	Overall      verdictView
	Totals       countView
	Scenarios    []scenarioView
}

type countView struct {
	Pass  int
	Fail  int
	Infra int
}

type verdictView struct {
	Verdict  string
	Icon     string
	Class    string
	ExitCode int
	Violations []string
}

type scenarioView struct {
	ID                   string
	Verdict              string
	Icon                 string
	Class                string
	Requested            int
	Completed            int
	Passes               int
	BehavioralFailures   int
	InfraErrors          int
	Observed             string
	CILow                string
	CIHigh               string
	HardConstraintFailed bool
	PassCount            int
	FailCount            int
	InfraCount           int
	PassPct              float64
	FailPct              float64
	InfraPct             float64
	Samples              []sampleView
}

type sampleView struct {
	SampleID    string
	Status      string
	StatusLabel string
	Category    string
	Message     string
	Kind        string
	KindLabel   string
	Icon        string
	Class       string
	Open        bool
	RunID       string
	TraceID     string
	PassAssert  int
	FailAssert  int
	Evaluations []evalView
	FixtureCalls []fixtureView
}

type evalView struct {
	Name        string
	Passed      bool
	Icon        string
	Class       string
	Criticality string
	Message     string
	Evidence    string
}

type fixtureView struct {
	Tool      string
	FixtureID string
	Mode      string
	Status    string
	Body      string
	Error     string
}

// WriteHTML atomically writes a self-contained HTML reliability report.
func WriteHTML(path string, report SuiteReport) error {
	if path == "" {
		path = "gust-report.html"
	}
	body, err := RenderHTML(report)
	if err != nil {
		return err
	}
	dir := filepath.Dir(path)
	if dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
	}
	tmp, err := os.CreateTemp(dir, ".gust-report-*.html")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	if _, err := tmp.WriteString(body); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmpName)
		return err
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmpName)
		return err
	}
	if err := os.Rename(tmpName, path); err != nil {
		_ = os.Remove(tmpName)
		return err
	}
	return nil
}

// RenderHTML returns the report as a self-contained HTML document.
func RenderHTML(report SuiteReport) (string, error) {
	view := buildView(report)
	var b bytes.Buffer
	if err := reportTmpl.ExecuteTemplate(&b, "report", view); err != nil {
		return "", err
	}
	return b.String(), nil
}

func buildView(report SuiteReport) htmlView {
	generated := report.GeneratedAt
	if generated.IsZero() {
		generated = time.Now().UTC()
	}
	view := htmlView{
		GeneratedAt:  generated.UTC().Format(time.RFC3339),
		EvaluationID: report.EvaluationID,
		PolicyName:   report.PolicyName,
		Overall: verdictView{
			Verdict:  string(report.Overall.OverallVerdict),
			Icon:     verdictIcon(string(report.Overall.OverallVerdict)),
			Class:    verdictClass(string(report.Overall.OverallVerdict)),
			ExitCode: report.Overall.ExitCode,
		},
	}
	for _, v := range report.Overall.Violations {
		view.Overall.Violations = append(view.Overall.Violations, v.Severity+": "+v.Message)
	}
	for _, res := range report.Results {
		if res == nil {
			continue
		}
		sc := buildScenario(res)
		view.Scenarios = append(view.Scenarios, sc)
		view.Totals.Pass += sc.PassCount
		view.Totals.Fail += sc.FailCount
		view.Totals.Infra += sc.InfraCount
	}
	return view
}

func buildScenario(res *api.ReliabilityResult) scenarioView {
	requested := res.SamplesRequested
	if requested == 0 {
		requested = res.Samples
	}
	completed := res.SamplesCompleted
	if completed == 0 && res.ExecutionErrors == 0 {
		completed = res.Samples
	}
	infra := res.InfrastructureErrors
	if infra == 0 {
		infra = res.ExecutionErrors
	}

	sc := scenarioView{
		ID:                   res.ScenarioID,
		Verdict:              string(res.Verdict),
		Icon:                 verdictIcon(string(res.Verdict)),
		Class:                verdictClass(string(res.Verdict)),
		Requested:            requested,
		Completed:            completed,
		Passes:               res.Passes,
		BehavioralFailures:   res.BehavioralFailures,
		InfraErrors:          infra,
		Observed:             fmt.Sprintf("%.3f", res.ObservedPassRate),
		CILow:                fmt.Sprintf("%.3f", res.ConfidenceInterval[0]),
		CIHigh:               fmt.Sprintf("%.3f", res.ConfidenceInterval[1]),
		HardConstraintFailed: res.HardConstraintFailed,
	}

	for _, s := range res.SampleResults {
		sv := buildSample(s)
		sc.Samples = append(sc.Samples, sv)
		switch sv.Kind {
		case "pass":
			sc.PassCount++
		case "fail":
			sc.FailCount++
		case "infra":
			sc.InfraCount++
		}
	}
	total := sc.PassCount + sc.FailCount + sc.InfraCount
	if total > 0 {
		sc.PassPct = 100 * float64(sc.PassCount) / float64(total)
		sc.FailPct = 100 * float64(sc.FailCount) / float64(total)
		sc.InfraPct = 100 * float64(sc.InfraCount) / float64(total)
	}
	return sc
}

func buildSample(s api.SampleResult) sampleView {
	kind, kindLabel, class, icon, open := sampleKind(s)
	sv := sampleView{
		SampleID:    s.SampleID,
		Status:      string(s.Status),
		StatusLabel: humanStatus(s.Status),
		Category:    string(s.FailureCategory),
		Message:     s.Message,
		Kind:        kind,
		KindLabel:   kindLabel,
		Icon:        icon,
		Class:       class,
		Open:        open,
		RunID:       s.RunID,
		TraceID:     s.TraceID,
	}
	for _, ev := range s.Evaluations {
		row := evalView{
			Name:        ev.EvaluatorName,
			Passed:      ev.Passed,
			Criticality: string(ev.Criticality),
			Message:     ev.Message,
		}
		if ev.Passed {
			row.Icon = "✓"
			row.Class = "pass"
			sv.PassAssert++
		} else {
			row.Icon = "✗"
			row.Class = "fail"
			sv.FailAssert++
		}
		if ev.Evidence != nil {
			row.Evidence = truncateRunes(fmt.Sprintf("%v", ev.Evidence), maxEvidenceRunes)
		}
		sv.Evaluations = append(sv.Evaluations, row)
	}
	for _, call := range s.FixtureCalls {
		fv := fixtureView{
			Tool:      call.Tool,
			FixtureID: call.FixtureID,
			Mode:      call.Mode,
			Status:    call.Status,
			Error:     call.Error,
		}
		if call.Body != nil {
			fv.Body = truncateRunes(fmt.Sprintf("%v", call.Body), maxEvidenceRunes)
		}
		sv.FixtureCalls = append(sv.FixtureCalls, fv)
	}
	return sv
}

func sampleKind(s api.SampleResult) (kind, label, class, icon string, open bool) {
	switch {
	case s.FailureCategory.IsInfrastructure() || s.Status == api.SampleStatusInfraError || s.Status == api.SampleStatusExcluded:
		return "infra", "Infrastructure", "warn", "!", true
	case s.Passed || s.Status == api.SampleStatusPassed:
		return "pass", "Passed", "pass", "✓", false
	default:
		return "fail", "Failed", "fail", "✗", true
	}
}

func humanStatus(s api.SampleExecutionStatus) string {
	if s == "" {
		return "unknown"
	}
	return strings.ReplaceAll(string(s), "_", " ")
}

func verdictIcon(v string) string {
	switch v {
	case string(api.VerdictPass):
		return "✓"
	case string(api.VerdictFail):
		return "✗"
	case string(api.VerdictFlaky):
		return "!"
	default:
		return "?"
	}
}

func verdictClass(v string) string {
	switch v {
	case string(api.VerdictPass):
		return "pass"
	case string(api.VerdictFail):
		return "fail"
	case string(api.VerdictFlaky):
		return "warn"
	default:
		return "info"
	}
}

func truncateRunes(s string, max int) string {
	r := []rune(s)
	if len(r) <= max {
		return s
	}
	return string(r[:max]) + "…"
}
