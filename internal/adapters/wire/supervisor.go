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

// PluginProcess manages the lifecycle of an external child process speaking JSON-RPC.
type PluginProcess struct {
	cmd      *exec.Cmd
	client   *Client
	manifest PluginManifest
	mu       sync.Mutex
	closed   bool
}

// StartPlugin launches an external executable with sanitized environment and timeout.
func StartPlugin(ctx context.Context, command string, args ...string) (*PluginProcess, error) {
	cmd := exec.CommandContext(ctx, command, args...)

	// Scrub environment: inherit only safe baseline PATH and OS vars
	cmd.Env = []string{
		"PATH=" + os.Getenv("PATH"),
		"SYSTEMROOT=" + os.Getenv("SYSTEMROOT"),
		"HOME=" + os.Getenv("HOME"),
		"USERPROFILE=" + os.Getenv("USERPROFILE"),
	}

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("failed to open stdin pipe: %w", err)
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		stdin.Close()
		return nil, fmt.Errorf("failed to open stdout pipe: %w", err)
	}

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

	// Fetch manifest with 5 second handshake timeout
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
