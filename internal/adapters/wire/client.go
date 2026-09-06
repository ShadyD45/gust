package wire

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sync"
	"sync/atomic"
)

var (
	ErrClientClosed   = errors.New("wire client closed")
	ErrInvalidJSONRPC = errors.New("invalid json-rpc response")
)

// Client conducts JSON-RPC 2.0 communication over standard in/out streams.
type Client struct {
	reader  *bufio.Reader
	writer  io.Writer
	mu      sync.Mutex
	reqID   int64
	pending map[int64]chan *Response
	pMu     sync.Mutex
	closed  bool
	closeCh chan struct{}
}

// NewClient initializes a Client over provided reader and writer.
func NewClient(r io.Reader, w io.Writer) *Client {
	c := &Client{
		reader:  bufio.NewReader(r),
		writer:  w,
		pending: make(map[int64]chan *Response),
		closeCh: make(chan struct{}),
	}
	go c.listen()
	return c
}

func (c *Client) listen() {
	for {
		line, err := c.reader.ReadBytes('\n')
		if err != nil {
			c.Close()
			return
		}
		var resp Response
		if err := json.Unmarshal(line, &resp); err != nil {
			continue
		}

		var id int64
		switch v := resp.ID.(type) {
		case float64:
			id = int64(v)
		case int64:
			id = v
		default:
			continue
		}

		c.pMu.Lock()
		ch, ok := c.pending[id]
		if ok {
			delete(c.pending, id)
		}
		c.pMu.Unlock()

		if ok {
			ch <- &resp
		}
	}
}

// Call executes a JSON-RPC method request and waits for response or context cancellation.
func (c *Client) Call(ctx context.Context, method string, params any, result any) error {
	id := atomic.AddInt64(&c.reqID, 1)

	var rawParams json.RawMessage
	if params != nil {
		data, err := json.Marshal(params)
		if err != nil {
			return fmt.Errorf("failed to marshal params: %w", err)
		}
		rawParams = data
	}

	req := Request{
		JSONRPC: "2.0",
		ID:      id,
		Method:  method,
		Params:  rawParams,
	}

	reqBytes, err := json.Marshal(req)
	if err != nil {
		return err
	}
	reqBytes = append(reqBytes, '\n')

	respCh := make(chan *Response, 1)
	c.pMu.Lock()
	if c.closed {
		c.pMu.Unlock()
		return ErrClientClosed
	}
	c.pending[id] = respCh
	c.pMu.Unlock()

	c.mu.Lock()
	_, err = c.writer.Write(reqBytes)
	c.mu.Unlock()
	if err != nil {
		c.pMu.Lock()
		delete(c.pending, id)
		c.pMu.Unlock()
		return fmt.Errorf("failed to write request: %w", err)
	}

	select {
	case <-ctx.Done():
		c.pMu.Lock()
		delete(c.pending, id)
		c.pMu.Unlock()
		return ctx.Err()
	case <-c.closeCh:
		return ErrClientClosed
	case resp := <-respCh:
		if resp.Error != nil {
			return resp.Error
		}
		if result != nil && len(resp.Result) > 0 {
			return json.Unmarshal(resp.Result, result)
		}
		return nil
	}
}

// Close terminates the client.
func (c *Client) Close() error {
	c.pMu.Lock()
	defer c.pMu.Unlock()
	if !c.closed {
		c.closed = true
		close(c.closeCh)
		for id, ch := range c.pending {
			close(ch)
			delete(c.pending, id)
		}
	}
	return nil
}
