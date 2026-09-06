package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
)

const gustYAML = `# gust project config
# Policy defaults used when --policy is omitted from analyze/test/compare.
policy: tests/policy.yaml
`

const defaultPolicyYAML = `version: "1.0"
name: default
hard_constraints:
  forbidden_tools: 0
  schema_violations: 0
reliability:
  default_minimum_pass_rate: 0.95
  min_samples_for_verdict: 5
  on_flaky: warn
`

const scenariosGitkeep = `# Place TestScenario YAML/JSON files here.
# Example: gust test tests/scenarios/ --runner synthetic --samples 20
`

const fixturesGitkeep = `# Place Fixture JSON files here (content-addressed tool responses).
`

const assertionsGitkeep = `# Optional shared assertion files referenced via assertions_files from scenarios.
`

func newInitCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "init [dir]",
		Short: "Scaffold a gust project (gust.yaml + tests/)",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			root := "."
			if len(args) == 1 {
				root = args[0]
			}
			return scaffoldProject(root)
		},
	}
	return cmd
}

func scaffoldProject(root string) error {
	abs, err := filepath.Abs(root)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(abs, 0o755); err != nil {
		return err
	}

	files := map[string]string{
		"gust.yaml":                    gustYAML,
		"tests/policy.yaml":            defaultPolicyYAML,
		"tests/scenarios/.gitkeep":     scenariosGitkeep,
		"tests/fixtures/.gitkeep":      fixturesGitkeep,
		"tests/assertions/.gitkeep":    assertionsGitkeep,
	}

	created := make([]string, 0, len(files))
	for rel, body := range files {
		path := filepath.Join(abs, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return err
		}
		if _, err := os.Stat(path); err == nil {
			fmt.Printf("skip (exists): %s\n", rel)
			continue
		} else if !os.IsNotExist(err) {
			return err
		}
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			return err
		}
		fmt.Printf("created: %s\n", rel)
		created = append(created, rel)
	}

	fmt.Printf("\nInitialized gust project in %s\n", abs)
	fmt.Println("Next steps:")
	fmt.Println("  1. Record an AgentRun (see docs/usage/integrate-your-app.md)")
	fmt.Println("  2. gust analyze <run.json> --policy tests/policy.yaml")
	fmt.Println("  3. gust scenario from-run <run.json> --output tests/scenarios/example.yaml")
	fmt.Println("  4. Add assertions, then: gust test tests/scenarios/ --runner synthetic --samples 20")
	fmt.Println("  5. gust mutate <golden-run.json>  # evaluator mutation score")
	if len(created) == 0 {
		fmt.Println("(all scaffold files already existed)")
	}
	return nil
}
