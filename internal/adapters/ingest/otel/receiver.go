package otel

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"gust/pkg/api"
)

// Receiver accepts live OTLP trace exports and maps them to AgentRuns.
// Mode 3 uses Wait(sampleID); `gust ingest otel serve` writes each completed run to disk.
type Receiver struct {
	cfg      ReceiverConfig
	httpSrv  *http.Server
	listener net.Listener
	addr     string
	grpc     *grpcState

	mu            sync.Mutex
	byID          map[string]*bufferedExport // sample_id or trace_id
	runs          map[string]api.AgentRun    // AgentRun ingest keyed by sample_id / run_id
	unmatched     []*bufferedExport          // OTLP exports with no gust.sample_id
	unmatchedRuns []api.AgentRun
	waiters       map[string][]chan struct{}
	waiterCount   int
}

type bufferedExport struct {
	payload  ExportPayload
	sampleID string
	traceID  string
}

// ReceiverConfig controls the live listener.
type ReceiverConfig struct {
	Listen      string        // HTTP bind address, e.g. ":4318" or "127.0.0.1:0"
	GRPCListen  string        // optional gRPC bind address, e.g. ":4317"
	IdleTimeout time.Duration // unused for single-POST completeness; reserved
	OnRun       func(api.AgentRun) error
}

// NewReceiver constructs a stopped receiver.
func NewReceiver(cfg ReceiverConfig) (*Receiver, error) {
	if cfg.Listen == "" {
		cfg.Listen = "127.0.0.1:4318"
	}
	ln, err := net.Listen("tcp", cfg.Listen)
	if err != nil {
		return nil, fmt.Errorf("otel receiver listen %s: %w", cfg.Listen, err)
	}
	r := &Receiver{
		cfg:      cfg,
		listener: ln,
		addr:     fmt.Sprintf("http://%s", ln.Addr().String()),
		byID:     make(map[string]*bufferedExport),
		runs:     make(map[string]api.AgentRun),
		waiters:  make(map[string][]chan struct{}),
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/traces", r.handleTraces)
	mux.HandleFunc("/v1/runs", r.handleRuns)
	mux.HandleFunc("/health", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
	r.httpSrv = &http.Server{
		Handler:      mux,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
	}
	return r, nil
}

// Start serves OTLP/HTTP (JSON and protobuf) in the background.
func (r *Receiver) Start() error {
	go func() { _ = r.httpSrv.Serve(r.listener) }()
	if r.cfg.GRPCListen != "" {
		if err := r.startGRPC(); err != nil {
			return err
		}
	}
	return nil
}

// Endpoint returns the HTTP base URL (no trailing slash).
func (r *Receiver) Endpoint() string { return r.addr }

// Close stops the HTTP (and gRPC) listeners.
func (r *Receiver) Close() error {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	r.stopGRPC()
	return r.httpSrv.Shutdown(ctx)
}

// Ingest records a parsed export and wakes any Wait callers.
func (r *Receiver) Ingest(payload ExportPayload) error {
	sampleID := SampleIDFromPayload(payload)
	ids := TraceIDs(payload)
	if len(ids) == 0 {
		return ErrEmptyPayload
	}
	r.mu.Lock()
	for _, tid := range ids {
		buf := &bufferedExport{payload: payload, sampleID: sampleID, traceID: tid}
		r.byID[tid] = buf
		if sampleID != "" {
			r.byID[sampleID] = buf
			r.signalLocked(sampleID)
		} else {
			r.unmatched = append(r.unmatched, buf)
			r.signalAllLocked()
		}
		r.signalLocked(tid)
	}
	r.mu.Unlock()

	if r.cfg.OnRun != nil {
		for _, tid := range ids {
			run, err := NewMapper().Map(payload, Options{TraceID: tid, RunID: firstNonEmpty(sampleID, tid)})
			if err != nil {
				return err
			}
			if sampleID != "" {
				if run.Metadata == nil {
					run.Metadata = map[string]any{}
				}
				run.Metadata["sample_id"] = sampleID
			}
			if err := r.cfg.OnRun(run); err != nil {
				return err
			}
		}
	}
	return nil
}

// Wait blocks until a matching AgentRun or OTLP export arrives.
// A missing gust.sample_id is accepted when this is the only in-flight waiter
// and exactly one untagged export is buffered (Mode 3 concurrency 1).
func (r *Receiver) Wait(ctx context.Context, sampleID string) (api.AgentRun, error) {
	if sampleID == "" {
		return api.AgentRun{}, fmt.Errorf("otel receiver: sample id is required")
	}
	r.mu.Lock()
	r.waiterCount++
	r.mu.Unlock()
	defer func() {
		r.mu.Lock()
		r.waiterCount--
		r.mu.Unlock()
	}()

	for {
		r.mu.Lock()
		if run, ok := r.claimRunLocked(sampleID); ok {
			r.mu.Unlock()
			return run, nil
		}
		if buf, ok := r.claimExportLocked(sampleID); ok {
			payload := buf.payload
			tid := buf.traceID
			r.mu.Unlock()
			run, err := NewMapper().Map(payload, Options{TraceID: tid, RunID: sampleID})
			if err != nil {
				return api.AgentRun{}, err
			}
			if run.Metadata == nil {
				run.Metadata = map[string]any{}
			}
			run.Metadata["sample_id"] = sampleID
			return run, nil
		}
		ch := make(chan struct{}, 1)
		r.waiters[sampleID] = append(r.waiters[sampleID], ch)
		r.mu.Unlock()

		select {
		case <-ctx.Done():
			return api.AgentRun{}, fmt.Errorf("otel receiver: timed out waiting for sample %s: %w", sampleID, ctx.Err())
		case <-ch:
		}
	}
}

func (r *Receiver) claimRunLocked(sampleID string) (api.AgentRun, bool) {
	if run, ok := r.runs[sampleID]; ok {
		return run, true
	}
	if r.waiterCount == 1 && len(r.unmatchedRuns) == 1 {
		run := r.unmatchedRuns[0]
		r.unmatchedRuns = nil
		if run.Metadata == nil {
			run.Metadata = map[string]any{}
		}
		run.Metadata["sample_id"] = sampleID
		r.runs[sampleID] = run
		return run, true
	}
	return api.AgentRun{}, false
}

func (r *Receiver) claimExportLocked(sampleID string) (*bufferedExport, bool) {
	if buf, ok := r.byID[sampleID]; ok {
		return buf, true
	}
	if r.waiterCount == 1 && len(r.unmatched) == 1 {
		buf := r.unmatched[0]
		r.unmatched = nil
		buf.sampleID = sampleID
		r.byID[sampleID] = buf
		return buf, true
	}
	return nil, false
}

func (r *Receiver) signalLocked(id string) {
	for _, ch := range r.waiters[id] {
		select {
		case ch <- struct{}{}:
		default:
		}
	}
	delete(r.waiters, id)
}

func (r *Receiver) signalAllLocked() {
	for id := range r.waiters {
		r.signalLocked(id)
	}
}

// IngestRun accepts a validated AgentRun (POST /v1/runs) and wakes Wait callers.
func (r *Receiver) IngestRun(run api.AgentRun) error {
	if err := run.Validate(); err != nil {
		return fmt.Errorf("ingest run: %w", err)
	}
	sampleID := ""
	if run.Metadata != nil {
		if s, ok := run.Metadata["sample_id"].(string); ok {
			sampleID = s
		}
	}
	r.mu.Lock()
	r.runs[run.RunID] = run
	if sampleID != "" {
		r.runs[sampleID] = run
		r.signalLocked(sampleID)
	} else {
		r.unmatchedRuns = append(r.unmatchedRuns, run)
		r.signalAllLocked()
	}
	r.signalLocked(run.RunID)
	r.mu.Unlock()

	if r.cfg.OnRun != nil {
		return r.cfg.OnRun(run)
	}
	return nil
}

func (r *Receiver) handleTraces(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	body, err := io.ReadAll(req.Body)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	var payload ExportPayload
	ct := req.Header.Get("Content-Type")
	if strings.Contains(ct, "protobuf") || strings.Contains(ct, "x-protobuf") {
		payload, err = payloadFromProtobuf(body)
	} else {
		if err = json.Unmarshal(body, &payload); err != nil {
			http.Error(w, fmt.Sprintf("malformed OTLP JSON: %v", err), http.StatusBadRequest)
			return
		}
	}
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if err := r.Ingest(payload); err != nil {
		http.Error(w, err.Error(), http.StatusUnprocessableEntity)
		return
	}
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(`{"partialSuccess":{}}`))
}

func (r *Receiver) handleRuns(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	body, err := io.ReadAll(req.Body)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	var run api.AgentRun
	if err := json.Unmarshal(body, &run); err != nil {
		http.Error(w, fmt.Sprintf("malformed AgentRun JSON: %v", err), http.StatusBadRequest)
		return
	}
	if err := r.IngestRun(run); err != nil {
		http.Error(w, err.Error(), http.StatusUnprocessableEntity)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	_, _ = w.Write([]byte(`{"ok":true}`))
}

// DeriveGRPCListen picks a gRPC bind address from the HTTP listen address.
// :4318 → :4317; ephemeral HTTP ports get an ephemeral gRPC port.
func DeriveGRPCListen(httpListen string) string {
	if httpListen == "" || strings.HasSuffix(httpListen, ":0") {
		return "127.0.0.1:0"
	}
	host, port, err := net.SplitHostPort(httpListen)
	if err != nil {
		return "127.0.0.1:0"
	}
	if port == "4318" {
		return net.JoinHostPort(host, "4317")
	}
	return "127.0.0.1:0"
}
