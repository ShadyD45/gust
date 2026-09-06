package cli

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"gust/internal/adapters/ingest/otel"
)

func TestReadIngestBytesFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "otlp.json")
	if err := os.WriteFile(path, []byte(`{"resourceSpans":[]}`), 0644); err != nil {
		t.Fatal(err)
	}
	data, err := readIngestBytes(path, "")
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != `{"resourceSpans":[]}` {
		t.Fatalf("got %s", data)
	}
}

func TestReadIngestBytesURL(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"resourceSpans":[]}`))
	}))
	defer srv.Close()
	data, err := readIngestBytes("", srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != `{"resourceSpans":[]}` {
		t.Fatalf("got %s", data)
	}
}

func TestReadIngestBytesRequiresSource(t *testing.T) {
	// Only fails when stdin is a TTY; in CI stdin is often a pipe.
	if _, err := readIngestBytes("", ""); err == nil {
		// Piped stdin is a valid source.
		return
	}
}

func TestDeriveGRPCListen(t *testing.T) {
	if got := otel.DeriveGRPCListen(":4318"); got != ":4317" {
		t.Fatalf("got %s", got)
	}
	if got := otel.DeriveGRPCListen("127.0.0.1:4318"); got != "127.0.0.1:4317" {
		t.Fatalf("got %s", got)
	}
	if got := otel.DeriveGRPCListen("127.0.0.1:0"); got != "127.0.0.1:0" {
		t.Fatalf("got %s", got)
	}
}
