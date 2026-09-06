package fixtures

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"sync"
	"time"

	"gust/internal/ports"
)

// MockToolProxyServer provides an ephemeral HTTP server to intercept agent tool calls.
type MockToolProxyServer struct {
	provider ports.FixtureProvider
	server   *http.Server
	listener net.Listener
	addr     string
	mu       sync.Mutex
	running  bool
}

// ToolCallRequest is the payload sent by live agents to call a mock tool.
type ToolCallRequest struct {
	Tool      string         `json:"tool"`
	Arguments map[string]any `json:"arguments"`
}

// ToolCallResponse is the response returned to the agent.
type ToolCallResponse struct {
	Status     string `json:"status"`
	StatusCode int    `json:"status_code,omitempty"`
	Body       any    `json:"body,omitempty"`
	Error      string `json:"error,omitempty"`
}

// NewMockToolProxyServer initializes a new server on loopback port 0.
func NewMockToolProxyServer(provider ports.FixtureProvider) (*MockToolProxyServer, error) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, fmt.Errorf("failed to bind loopback listener: %w", err)
	}

	proxy := &MockToolProxyServer{
		provider: provider,
		listener: listener,
		addr:     fmt.Sprintf("http://%s", listener.Addr().String()),
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/v1/tools/call", proxy.handleToolCall)
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})

	proxy.server = &http.Server{
		Handler:      mux,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
	}

	return proxy, nil
}

// Start launches the server in a background goroutine and returns the endpoint URL.
func (s *MockToolProxyServer) Start() string {
	s.mu.Lock()
	if s.running {
		s.mu.Unlock()
		return s.addr
	}
	s.running = true
	s.mu.Unlock()

	go func() {
		_ = s.server.Serve(s.listener)
	}()

	return s.addr
}

// Endpoint returns the base HTTP URL of the server.
func (s *MockToolProxyServer) Endpoint() string {
	return s.addr
}

// Close gracefully shuts down the server.
func (s *MockToolProxyServer) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.running {
		return nil
	}
	s.running = false
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	return s.server.Shutdown(ctx)
}

func (s *MockToolProxyServer) handleToolCall(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}

	var req ToolCallRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON body: "+err.Error(), http.StatusBadRequest)
		return
	}

	call := ports.ToolCall{
		Name:      req.Tool,
		Arguments: req.Arguments,
	}

	resp, found, err := s.provider.Lookup(r.Context(), call)
	if err != nil {
		http.Error(w, "Fixture resolution error: "+err.Error(), http.StatusInternalServerError)
		return
	}

	if !found {
		w.WriteHeader(http.StatusNotFound)
		_ = json.NewEncoder(w).Encode(ToolCallResponse{
			Status: "error",
			Error:  fmt.Sprintf("no fixture found for tool %q", req.Tool),
		})
		return
	}

	statusCode := http.StatusOK
	if resp.StatusCode > 0 {
		statusCode = resp.StatusCode
	}
	w.WriteHeader(statusCode)

	toolResp := ToolCallResponse{
		Status:     resp.Status,
		StatusCode: statusCode,
		Body:       resp.Body,
		Error:      resp.Error,
	}
	_ = json.NewEncoder(w).Encode(toolResp)
}
