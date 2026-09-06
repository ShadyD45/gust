package wire

import "encoding/json"

// JSON-RPC 2.0 Request
type Request struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      any             `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

// JSON-RPC 2.0 Response
type Response struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      any             `json:"id,omitempty"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *RPCError       `json:"error,omitempty"`
}

type RPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data,omitempty"`
}

func (e *RPCError) Error() string {
	return e.Message
}

// Plugin Manifest returned on "manifest" call
type PluginManifest struct {
	ProtocolVersion string   `json:"protocol_version"`
	Kind            string   `json:"kind"` // "evaluator", "mutator", "fixture_provider", "test_runner"
	Name            string   `json:"name"`
	Version         string   `json:"version"`
	Description     string   `json:"description,omitempty"`
	Capabilities    []string `json:"capabilities,omitempty"`
}
