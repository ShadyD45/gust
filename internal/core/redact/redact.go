package redact

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"gust/pkg/api"
)

// Rule replaces sensitive substrings in string leaves.
type Rule struct {
	Name        string
	Pattern     *regexp.Regexp
	Replacement string
}

// DefaultRules returns built-in email, phone, and token redactors.
func DefaultRules() []Rule {
	return []Rule{
		{Name: "email", Pattern: regexp.MustCompile(`[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}`), Replacement: "[REDACTED_EMAIL]"},
		// Require a separator-heavy phone-like run; avoid ISO-8601 timestamps.
		{Name: "phone", Pattern: regexp.MustCompile(`(?:\+?\d{1,3}[\s\-.]?)?(?:\(?\d{3}\)?[\s\-.]?)\d{3}[\s\-.]?\d{4}\b`), Replacement: "[REDACTED_PHONE]"},
		{Name: "jwt", Pattern: regexp.MustCompile(`\beyJ[A-Za-z0-9_-]+\.[A-Za-z0-9_-]+\.[A-Za-z0-9_-]+\b`), Replacement: "[REDACTED_JWT]"},
		{Name: "bearer", Pattern: regexp.MustCompile(`(?i)\b(bearer\s+)[A-Za-z0-9\-._~+/]+=*`), Replacement: "${1}[REDACTED_TOKEN]"},
		{Name: "api_key", Pattern: regexp.MustCompile(`(?i)\b(?:sk|api[_-]?key|token)[-_]?[A-Za-z0-9]{16,}`), Replacement: "[REDACTED_API_KEY]"},
		{Name: "cardish", Pattern: regexp.MustCompile(`\b(?:\d[ -]*?){13,19}\b`), Replacement: "[REDACTED_DIGITS]"},
	}
}

// ParseCustomRules compiles user regex rules from "name:pattern:replacement" triples
// or JSON objects {"name","pattern","replacement"}.
func ParseCustomRules(specs []string) ([]Rule, error) {
	out := make([]Rule, 0, len(specs))
	for _, s := range specs {
		s = strings.TrimSpace(s)
		if s == "" {
			continue
		}
		if strings.HasPrefix(s, "{") {
			var raw struct {
				Name        string `json:"name"`
				Pattern     string `json:"pattern"`
				Replacement string `json:"replacement"`
			}
			if err := json.Unmarshal([]byte(s), &raw); err != nil {
				return nil, fmt.Errorf("custom rule JSON: %w", err)
			}
			re, err := regexp.Compile(raw.Pattern)
			if err != nil {
				return nil, fmt.Errorf("custom rule %q: %w", raw.Name, err)
			}
			rep := raw.Replacement
			if rep == "" {
				rep = "[REDACTED]"
			}
			out = append(out, Rule{Name: raw.Name, Pattern: re, Replacement: rep})
			continue
		}
		parts := strings.SplitN(s, ":", 3)
		if len(parts) < 2 {
			return nil, fmt.Errorf("custom rule %q: want name:pattern[:replacement]", s)
		}
		re, err := regexp.Compile(parts[1])
		if err != nil {
			return nil, fmt.Errorf("custom rule %q: %w", parts[0], err)
		}
		rep := "[REDACTED]"
		if len(parts) == 3 && parts[2] != "" {
			rep = parts[2]
		}
		out = append(out, Rule{Name: parts[0], Pattern: re, Replacement: rep})
	}
	return out, nil
}

// Config controls AgentRun redaction.
type Config struct {
	Enabled    bool
	Rules      []Rule
	OptOutNote string // recorded when Enabled=false for audit
}

// Result summarizes what was scrubbed.
type Result struct {
	Enabled       bool           `json:"enabled"`
	OptOutNote    string         `json:"opt_out_note,omitempty"`
	Replacements  map[string]int `json:"replacements,omitempty"`
	TotalReplaced int            `json:"total_replaced"`
}

// RedactRun deep-copies and redacts string leaves in an AgentRun.
func RedactRun(run api.AgentRun, cfg Config) (api.AgentRun, Result, error) {
	res := Result{Enabled: cfg.Enabled, OptOutNote: cfg.OptOutNote, Replacements: map[string]int{}}
	if !cfg.Enabled {
		return run, res, nil
	}
	rules := cfg.Rules
	if len(rules) == 0 {
		rules = DefaultRules()
	}

	raw, err := json.Marshal(run)
	if err != nil {
		return run, res, err
	}
	var tree any
	if err := json.Unmarshal(raw, &tree); err != nil {
		return run, res, err
	}
	tree = walk(tree, rules, res.Replacements)
	for _, n := range res.Replacements {
		res.TotalReplaced += n
	}
	outRaw, err := json.Marshal(tree)
	if err != nil {
		return run, res, err
	}
	var out api.AgentRun
	if err := json.Unmarshal(outRaw, &out); err != nil {
		return run, res, err
	}
	return out, res, nil
}

func walk(v any, rules []Rule, counts map[string]int) any {
	switch t := v.(type) {
	case string:
		if looksLikeTimestamp(t) {
			return t
		}
		return applyRules(t, rules, counts)
	case map[string]any:
		for k, child := range t {
			if skipRedactKey(k) {
				continue
			}
			t[k] = walk(child, rules, counts)
		}
		return t
	case []any:
		for i, child := range t {
			t[i] = walk(child, rules, counts)
		}
		return t
	default:
		return v
	}
}

func skipRedactKey(k string) bool {
	switch strings.ToLower(k) {
	case "start_time", "end_time", "extracted_at", "schema_version", "run_id", "span_id", "trace_id":
		return true
	default:
		return false
	}
}

var isoTimestamp = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}T`)

func looksLikeTimestamp(s string) bool {
	return isoTimestamp.MatchString(s)
}

func applyRules(s string, rules []Rule, counts map[string]int) string {
	out := s
	for _, r := range rules {
		if r.Pattern == nil {
			continue
		}
		n := 0
		out = r.Pattern.ReplaceAllStringFunc(out, func(m string) string {
			n++
			return r.Pattern.ReplaceAllString(m, r.Replacement)
		})
		if n > 0 {
			counts[r.Name] += n
		}
	}
	return out
}
