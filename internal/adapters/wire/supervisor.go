package wire

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"sync"
	"time"

	"gust/internal/ports"
	"gust/pkg/api"
)

// baselineEnvVars are the only variables forwarded to deterministic plugin processes.
// They cover interpreter startup (PATH, temp dirs, user profile) on both
// POSIX and Windows without exposing application credentials.
var baselineEnvVars = []string{
	"PATH",
	"HOME",
	"LANG",
	"TMPDIR",
	"SYSTEMROOT",
	"SYSTEMDRIVE",
	"USERPROFILE",
	"LOCALAPPDATA",
	"APPDATA",
	"PATHEXT",
	"COMSPEC",
	"TEMP",
	"TMP",
}

// judgeEnvAllowlist is additionally forwarded for LLM-judge plugins so official
// provider SDKs (openai, anthropic, google-genai, ollama) can authenticate.
var judgeEnvAllowlist = []string{
	"OPENAI_API_KEY",
	"OPENAI_BASE_URL",
	"ANTHROPIC_API_KEY",
	"GOOGLE_API_KEY",
	"GEMINI_API_KEY",
	"GOOGLE_GENAI_API_KEY",
	"GUST_JUDGE_API_KEY",
	"GUST_JUDGE_MODEL",
	"GUST_JUDGE_PROVIDER",
	"GUST_JUDGE_ENDPOINT",
	"OLLAMA_HOST",
	"OLLAMA_API_KEY",
}

func scrubbedEnv() []string {
	env := make([]string, 0, len(baselineEnvVars))
	for _, key := range baselineEnvVars {
		if value, ok := os.LookupEnv(key); ok {
			env = append(env, key+"="+value)
		}
	}
	return env
}

func envWithAllowlist(allow []string) []string {
	env := scrubbedEnv()
	for _, key := range allow {
		if value, ok := os.LookupEnv(key); ok {
			env = append(env, key+"="+value)
		}
	}
	return env
}

// PluginProcess manages the lifecycle of an external child process speaking JSON-RPC.
type PluginProcess struct {
	cmd      *exec.Cmd
	client   *Client
	manifest PluginManifest
	mu       sync.Mutex
	closed   bool
}

// StartPlugin launches an external executable with sanitized environment and timeout.
// Credentials are not forwarded — use StartJudgePlugin for LLM judges that need API keys.
func StartPlugin(ctx context.Context, command string, args ...string) (*PluginProcess, error) {
	return startPlugin(ctx, scrubbedEnv(), command, args...)
}

// StartJudgePlugin launches a plugin with judge API-key env vars forwarded.
func StartJudgePlugin(ctx context.Context, command string, args ...string) (*PluginProcess, error) {
	return startPlugin(ctx, envWithAllowlist(judgeEnvAllowlist), command, args...)
}

func startPlugin(ctx context.Context, env []string, command string, args ...string) (*PluginProcess, error) {
	cmd := exec.CommandContext(ctx, command, args...)
	cmd.Env = env

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("failed to open stdin pipe: %w", err)
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		stdin.Close()
		return nil, fmt.Errorf("failed to open stdout pipe: %w", err)
	}

	// stdout carries the protocol, so plugin diagnostics belong on stderr.
	cmd.Stderr = os.Stderr

	if err := cmd.Start(); err != nil {
		stdin.Close()
		stdout.Close()
		return nil, fmt.Errorf("failed to start plugin process: %w", err)
	}

	client := NewClient(stdout, stdin)
	p := &PluginProcess{
		cmd:    cmd,
		client: client,
	}

	handshakeCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	var manifest PluginManifest
	if err := client.Call(handshakeCtx, "manifest", nil, &manifest); err != nil {
		p.Close()
		return nil, fmt.Errorf("plugin manifest handshake failed: %w", err)
	}
	p.manifest = manifest

	return p, nil
}

func (p *PluginProcess) Manifest() PluginManifest {
	return p.manifest
}

func (p *PluginProcess) Client() *Client {
	return p.client
}

func (p *PluginProcess) Close() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.closed {
		return nil
	}
	p.closed = true
	_ = p.client.Close()
	if p.cmd != nil && p.cmd.Process != nil {
		_ = p.cmd.Process.Kill()
	}
	return nil
}

// WireEvaluator adapts a PluginProcess into a native ports.Evaluator.
type WireEvaluator struct {
	proc *PluginProcess
}

func NewWireEvaluator(proc *PluginProcess) *WireEvaluator {
	return &WireEvaluator{proc: proc}
}

func (we *WireEvaluator) Name() string    { return we.proc.manifest.Name }
func (we *WireEvaluator) Version() string { return we.proc.manifest.Version }

func (we *WireEvaluator) Evaluate(ctx context.Context, run api.AgentRun, expected *api.Assertion, evalCtx ports.EvaluationContext) (ports.EvaluationResult, error) {
	params := map[string]any{
		"run":      run,
		"expected": expected,
		"context":  evalCtx,
	}

	var res ports.EvaluationResult
	err := we.proc.client.Call(ctx, "evaluate", params, &res)
	return res, err
}
