package langfuse

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

const (
	EnvHost      = "LANGFUSE_HOST"
	EnvPublicKey = "LANGFUSE_PUBLIC_KEY"
	EnvSecretKey = "LANGFUSE_SECRET_KEY"
	DefaultHost  = "https://cloud.langfuse.com"
)

// Config is env-first; flags override when non-empty.
type Config struct {
	Host      string
	PublicKey string
	SecretKey string
	Timeout   time.Duration
}

func (c Config) withDefaults() Config {
	if c.Host == "" {
		c.Host = os.Getenv(EnvHost)
	}
	if c.Host == "" {
		c.Host = DefaultHost
	}
	if c.PublicKey == "" {
		c.PublicKey = os.Getenv(EnvPublicKey)
	}
	if c.SecretKey == "" {
		c.SecretKey = os.Getenv(EnvSecretKey)
	}
	if c.Timeout <= 0 {
		c.Timeout = 30 * time.Second
	}
	c.Host = strings.TrimRight(c.Host, "/")
	return c
}

func (c Config) validate() error {
	if c.PublicKey == "" || c.SecretKey == "" {
		return fmt.Errorf("langfuse: set %s and %s (or --public-key / --secret-key)", EnvPublicKey, EnvSecretKey)
	}
	return nil
}

// Client talks to the Langfuse public REST API.
type Client struct {
	cfg    Config
	client *http.Client
}

func NewClient(cfg Config) (*Client, error) {
	cfg = cfg.withDefaults()
	if err := cfg.validate(); err != nil {
		return nil, err
	}
	return &Client{cfg: cfg, client: &http.Client{Timeout: cfg.Timeout}}, nil
}

// FetchTrace loads one trace by id.
func (c *Client) FetchTrace(id string) (Trace, error) {
	if id == "" {
		return Trace{}, fmt.Errorf("langfuse: --trace-id is required")
	}
	var tr Trace
	if err := c.get("/api/public/traces/"+url.PathEscape(id), &tr); err != nil {
		return Trace{}, err
	}
	return tr, nil
}

// ListSessionTraceIDs returns trace ids in a session.
func (c *Client) ListSessionTraceIDs(sessionID string) ([]string, error) {
	if sessionID == "" {
		return nil, fmt.Errorf("langfuse: --session is required")
	}
	q := url.Values{"sessionId": {sessionID}}
	var list TraceList
	if err := c.get("/api/public/traces?"+q.Encode(), &list); err != nil {
		return nil, err
	}
	ids := make([]string, 0, len(list.Data))
	for _, tr := range list.Data {
		if tr.ID != "" {
			ids = append(ids, tr.ID)
		}
	}
	return ids, nil
}

func (c *Client) get(path string, dest any) error {
	req, err := http.NewRequest(http.MethodGet, c.cfg.Host+path, nil)
	if err != nil {
		return err
	}
	req.SetBasicAuth(c.cfg.PublicKey, c.cfg.SecretKey)
	req.Header.Set("Accept", "application/json")
	resp, err := c.client.Do(req)
	if err != nil {
		return fmt.Errorf("langfuse: %w", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	if resp.StatusCode >= 300 {
		return fmt.Errorf("langfuse: HTTP %d: %s", resp.StatusCode, truncate(body, 256))
	}
	if err := json.Unmarshal(body, dest); err != nil {
		return fmt.Errorf("langfuse: decode: %w", err)
	}
	return nil
}

func truncate(b []byte, n int) string {
	if len(b) <= n {
		return string(b)
	}
	return string(b[:n]) + "…"
}
