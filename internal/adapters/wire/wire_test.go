package wire

import (
	"bufio"
	"context"
	"encoding/json"
	"io"
	"strings"
	"testing"
	"time"

	"gust/internal/ports"
	"gust/pkg/api"
)

func TestWireClientCommunication(t *testing.T) {
	clientRead, serverWrite := io.Pipe()
	serverRead, clientWrite := io.Pipe()

	client := NewClient(clientRead, clientWrite)
	defer client.Close()

	// Mock server listening on serverRead and responding to serverWrite
	go func() {
		scanner := bufio.NewScanner(serverRead)
		for scanner.Scan() {
			var req Request
			if err := json.Unmarshal(scanner.Bytes(), &req); err != nil {
				continue
			}

			switch req.Method {
			case "manifest":
				manifest := PluginManifest{
					ProtocolVersion: "1.0",
					Kind:            "evaluator",
					Name:            "remote_eval",
					Version:         "0.1.0",
				}
				data, _ := json.Marshal(manifest)
				resp := Response{
					JSONRPC: "2.0",
					ID:      req.ID,
					Result:  data,
				}
				respBytes, _ := json.Marshal(resp)
				respBytes = append(respBytes, '\n')
				serverWrite.Write(respBytes)

			case "evaluate":
				evalRes := ports.EvaluationResult{
					EvaluatorName: "remote_eval",
					Passed:        true,
					Score:         1.0,
					Message:       "Evaluation passed remotely",
				}
				data, _ := json.Marshal(evalRes)
				resp := Response{
					JSONRPC: "2.0",
					ID:      req.ID,
					Result:  data,
				}
				respBytes, _ := json.Marshal(resp)
				respBytes = append(respBytes, '\n')
				serverWrite.Write(respBytes)
			}
		}
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	// 1. Call manifest
	var manifest PluginManifest
	if err := client.Call(ctx, "manifest", nil, &manifest); err != nil {
		t.Fatalf("client.Call(manifest) failed: %v", err)
	}
	if manifest.Name != "remote_eval" || manifest.Kind != "evaluator" {
		t.Errorf("manifest mismatch: %+v", manifest)
	}

	// 2. Call evaluate via WireEvaluator
	proc := &PluginProcess{
		client:   client,
		manifest: manifest,
	}
	wireEval := NewWireEvaluator(proc)

	run := api.AgentRun{
		SchemaVersion: api.SchemaVersion,
		RunID:         "test_run",
		Agent:         api.AgentInfo{Name: "agent", Version: "1.0"},
		Task:          api.TaskInfo{ID: "t1", Input: "test input"},
		Outcome:       api.RunOutcome{Status: "completed"},
	}

	res, err := wireEval.Evaluate(ctx, run, nil, ports.EvaluationContext{})
	if err != nil {
		t.Fatalf("wireEval.Evaluate failed: %v", err)
	}
	if !res.Passed || res.Score != 1.0 {
		t.Errorf("unexpected evaluation result: %+v", res)
	}
}

func TestWireClientTimeout(t *testing.T) {
	clientRead, _ := io.Pipe()
	defer clientRead.Close()
	client := NewClient(clientRead, io.Discard)
	defer client.Close()

	// Timeout quickly
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	var res map[string]any
	err := client.Call(ctx, "never_responds", nil, &res)
	if err != context.DeadlineExceeded {
		t.Errorf("expected context.DeadlineExceeded, got %v", err)
	}
}

func TestWireClientCloseNoPanic(t *testing.T) {
	for i := 0; i < 100; i++ {
		clientRead, serverWrite := io.Pipe()
		serverRead, clientWrite := io.Pipe()
		client := NewClient(clientRead, clientWrite)

		go func() {
			buf := make([]byte, 1)
			for {
				if _, err := serverRead.Read(buf); err != nil {
					return
				}
			}
		}()
		_ = serverWrite

		done := make(chan error, 1)
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()
			done <- client.Call(ctx, "hang", nil, nil)
		}()

		time.Sleep(time.Millisecond)
		_ = client.Close()
		err := <-done
		if err != nil && err != ErrClientClosed && err != context.DeadlineExceeded && err != context.Canceled {
			t.Fatalf("iteration %d: unexpected error %v", i, err)
		}
		_ = clientRead.Close()
		_ = clientWrite.Close()
		_ = serverRead.Close()
	}
}

func TestReadLineLimited(t *testing.T) {
	r := bufio.NewReader(strings.NewReader("ok\n"))
	line, err := readLineLimited(r, 16)
	if err != nil || string(line) != "ok\n" {
		t.Fatalf("got %q err=%v", line, err)
	}

	huge := strings.Repeat("a", 64) + "\n"
	r2 := bufio.NewReaderSize(strings.NewReader(huge), 8)
	_, err = readLineLimited(r2, 32)
	if err != ErrLineTooLong {
		t.Fatalf("expected ErrLineTooLong, got %v", err)
	}
}

func TestWireClientLineTooLongCloses(t *testing.T) {
	old := maxLineBytes
	maxLineBytes = 64
	defer func() { maxLineBytes = old }()

	clientRead, serverWrite := io.Pipe()
	serverRead, clientWrite := io.Pipe()
	go io.Copy(io.Discard, serverRead)

	client := NewClient(clientRead, clientWrite)
	defer func() { _ = client.Close() }()

	go func() {
		huge := append([]byte(strings.Repeat("a", 128)), '\n')
		_, _ = serverWrite.Write(huge)
		_ = serverWrite.Close()
	}()

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
		err := client.Call(ctx, "ping", nil, nil)
		cancel()
		if err == ErrClientClosed {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatal("expected client to close after oversized line")
}
