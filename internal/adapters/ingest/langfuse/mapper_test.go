package langfuse

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func goldenTrace(t *testing.T) Trace {
	t.Helper()
	path := filepath.Join("..", "..", "..", "..", "testdata", "langfuse", "trace.json")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var tr Trace
	if err := json.Unmarshal(data, &tr); err != nil {
		t.Fatal(err)
	}
	return tr
}

func TestMapGoldenTrace(t *testing.T) {
	run, err := Map(goldenTrace(t), Options{})
	if err != nil {
		t.Fatal(err)
	}
	if run.RunID != "4bf92f3577b34da6a3ce929d0e0e4736" {
		t.Fatalf("run id %s", run.RunID)
	}
	if run.Agent.Name != "support-agent" || run.Agent.Version != "1.4" {
		t.Fatalf("agent %+v", run.Agent)
	}
	if run.Task.ID != "refund-001" || run.Task.Input != "Cancel my latest order" {
		t.Fatalf("task %+v", run.Task)
	}
	if run.Outcome.Output != "Order 123 cancelled successfully." {
		t.Fatalf("output %s", run.Outcome.Output)
	}
	if len(run.Trace) != 3 {
		t.Fatalf("spans %d", len(run.Trace))
	}
	tool := run.Trace[1]
	if tool.Type != "tool" || tool.Name != "get_orders" {
		t.Fatalf("tool span %+v", tool)
	}
	in, _ := tool.Attributes["input"].(map[string]any)
	if in["customer_id"] != float64(42) {
		t.Fatalf("input %+v", tool.Attributes["input"])
	}
}

func TestClientFetchTrace(t *testing.T) {
	tr := goldenTrace(t)
	body, _ := json.Marshal(tr)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/public/traces/"+tr.ID {
			t.Errorf("path %s", r.URL.Path)
		}
		user, pass, ok := r.BasicAuth()
		if !ok || user != "pk" || pass != "sk" {
			t.Errorf("auth %s %s", user, pass)
		}
		_, _ = w.Write(body)
	}))
	defer srv.Close()

	c, err := NewClient(Config{Host: srv.URL, PublicKey: "pk", SecretKey: "sk"})
	if err != nil {
		t.Fatal(err)
	}
	got, err := c.FetchTrace(tr.ID)
	if err != nil {
		t.Fatal(err)
	}
	run, err := Map(got, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if run.Task.Input != "Cancel my latest order" {
		t.Fatalf("input %s", run.Task.Input)
	}
}

func TestClientListSession(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("sessionId") != "refund-001" {
			t.Errorf("query %s", r.URL.RawQuery)
		}
		_ = json.NewEncoder(w).Encode(TraceList{Data: []Trace{{ID: "aaa"}, {ID: "bbb"}}})
	}))
	defer srv.Close()
	c, err := NewClient(Config{Host: srv.URL, PublicKey: "pk", SecretKey: "sk"})
	if err != nil {
		t.Fatal(err)
	}
	ids, err := c.ListSessionTraceIDs("refund-001")
	if err != nil {
		t.Fatal(err)
	}
	if len(ids) != 2 || ids[0] != "aaa" {
		t.Fatalf("%v", ids)
	}
}

func TestNewClientRequiresKeys(t *testing.T) {
	t.Setenv(EnvPublicKey, "")
	t.Setenv(EnvSecretKey, "")
	if _, err := NewClient(Config{Host: "http://example"}); err == nil {
		t.Fatal("expected missing keys")
	}
}
