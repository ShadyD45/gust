package fixtures

import (
	"context"
	"fmt"
	"sync"
	"time"

	"gust/internal/ports"
	"gust/pkg/api"
	"gust/pkg/jcs"
)

// MemoryFixtureProvider provides thread-safe in-memory resolution of fixtures.
type MemoryFixtureProvider struct {
	mu            sync.Mutex
	exactFixtures map[string]api.Fixture   // key: tool_name + ":" + input_hash
	seqFixtures   map[string][]api.Fixture // key: tool_name -> list of sequential fixtures
	seqCounters   map[string]int           // key: tool_name -> current sequence index
	ledger        []api.FixtureCallEvidence
}

// NewMemoryFixtureProvider creates an empty MemoryFixtureProvider.
func NewMemoryFixtureProvider() *MemoryFixtureProvider {
	return &MemoryFixtureProvider{
		exactFixtures: make(map[string]api.Fixture),
		seqFixtures:   make(map[string][]api.Fixture),
		seqCounters:   make(map[string]int),
	}
}

// LoadFixtures loads a slice of fixtures into the provider.
func (p *MemoryFixtureProvider) LoadFixtures(fixtures []api.Fixture) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	for _, fx := range fixtures {
		if fx.MatchStrategy == api.MatchStrategyOrderedSequence {
			p.seqFixtures[fx.Tool] = append(p.seqFixtures[fx.Tool], fx)
		} else {
			// exact_hash or prefer_exact_then_sequence
			hash := fx.InputHash
			if hash == "" && fx.RecordedInput != nil {
				h, err := jcs.ContentHash(fx.RecordedInput)
				if err == nil {
					hash = h
				}
			}
			key := fmt.Sprintf("%s:%s", fx.Tool, hash)
			p.exactFixtures[key] = fx
		}
	}
	return nil
}

// Lookup resolves a tool call to its recorded response according to match strategy and failure mode.
func (p *MemoryFixtureProvider) Lookup(ctx context.Context, call ports.ToolCall) (api.RecordedResponse, bool, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	var matchedFx *api.Fixture

	// 1. Try exact hash match
	hash, err := jcs.ContentHash(call.Arguments)
	if err == nil {
		key := fmt.Sprintf("%s:%s", call.Name, hash)
		if fx, ok := p.exactFixtures[key]; ok {
			matchedFx = &fx
		}
	}

	// 2. If not matched, try ordered sequence
	if matchedFx == nil {
		if seq, ok := p.seqFixtures[call.Name]; ok {
			idx := p.seqCounters[call.Name]
			if idx < len(seq) {
				matchedFx = &seq[idx]
				p.seqCounters[call.Name]++
			}
		}
	}

	if matchedFx == nil {
		p.ledger = append(p.ledger, api.FixtureCallEvidence{
			Tool:      call.Name,
			Arguments: call.Arguments,
			Found:     false,
			Status:    "error",
			Error:     fmt.Sprintf("no fixture found for tool %q", call.Name),
		})
		return api.RecordedResponse{}, false, nil
	}

	// 3. Apply failure mode and delay if configured
	if matchedFx.DelayMs > 0 {
		select {
		case <-ctx.Done():
			return api.RecordedResponse{}, false, ctx.Err()
		case <-time.After(time.Duration(matchedFx.DelayMs) * time.Millisecond):
		}
	}

	mode := string(matchedFx.Mode)
	if mode == "" {
		mode = string(api.FailureModeSuccess)
	}

	switch matchedFx.Mode {
	case api.FailureModeTimeout:
		// Hang until context deadline
		<-ctx.Done()
		p.ledger = append(p.ledger, api.FixtureCallEvidence{
			Tool:      call.Name,
			Arguments: call.Arguments,
			FixtureID: matchedFx.FixtureID,
			Mode:      mode,
			Found:     true,
			Status:    "error",
			Error:     ctx.Err().Error(),
		})
		return api.RecordedResponse{}, false, ctx.Err()

	case api.FailureModePartialFailure:
		resp := api.RecordedResponse{
			Status:     "error",
			StatusCode: 500,
			Error:      "injected simulated server error (partial_failure)",
		}
		p.appendLedger(call, *matchedFx, mode, resp, true)
		return resp, true, nil

	case api.FailureModeMalformed:
		resp := api.RecordedResponse{
			Status: "success",
			Body:   `{"broken": truncated json...`,
		}
		p.appendLedger(call, *matchedFx, mode, resp, true)
		return resp, true, nil

	default:
		// Normal success or configured recorded response
		p.appendLedger(call, *matchedFx, mode, matchedFx.RecordedResponse, true)
		return matchedFx.RecordedResponse, true, nil
	}
}

func (p *MemoryFixtureProvider) appendLedger(call ports.ToolCall, fx api.Fixture, mode string, resp api.RecordedResponse, found bool) {
	p.ledger = append(p.ledger, api.FixtureCallEvidence{
		Tool:       call.Name,
		Arguments:  call.Arguments,
		FixtureID:  fx.FixtureID,
		Mode:       mode,
		Status:     resp.Status,
		StatusCode: resp.StatusCode,
		Body:       resp.Body,
		Error:      resp.Error,
		Found:      found,
	})
}

// CallLedger returns a copy of recorded fixture resolutions for this provider.
func (p *MemoryFixtureProvider) CallLedger() []api.FixtureCallEvidence {
	p.mu.Lock()
	defer p.mu.Unlock()
	out := make([]api.FixtureCallEvidence, len(p.ledger))
	copy(out, p.ledger)
	return out
}

// Record saves a new tool call and response into the provider.
func (p *MemoryFixtureProvider) Record(ctx context.Context, call ports.ToolCall, resp api.RecordedResponse) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	hash, err := jcs.ContentHash(call.Arguments)
	if err != nil {
		return err
	}

	fx := api.Fixture{
		FixtureID:        fmt.Sprintf("fx_rec_%s_%d", call.Name, time.Now().UnixNano()),
		Tool:             call.Name,
		InputHash:        hash,
		MatchStrategy:    api.MatchStrategyExactHash,
		RecordedInput:    call.Arguments,
		RecordedResponse: resp,
		Provenance:       api.ProvenanceRecorded,
	}

	key := fmt.Sprintf("%s:%s", call.Name, hash)
	p.exactFixtures[key] = fx
	return nil
}

// Replace drops all fixtures and loads the given set. Used between suite scenarios
// so one mock proxy can serve isolated fixture worlds.
func (p *MemoryFixtureProvider) Replace(fixtures []api.Fixture) error {
	p.mu.Lock()
	p.exactFixtures = make(map[string]api.Fixture)
	p.seqFixtures = make(map[string][]api.Fixture)
	p.seqCounters = make(map[string]int)
	p.mu.Unlock()
	if len(fixtures) == 0 {
		return nil
	}
	return p.LoadFixtures(fixtures)
}

// Reset clears sequence counters and the call ledger for stateful mocks.
func (p *MemoryFixtureProvider) Reset() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.seqCounters = make(map[string]int)
	p.ledger = nil
	return nil
}

// Clone copies fixtures into a new provider with reset sequence counters.
// Concurrent Mode 3 samples should each get their own clone so ordered
// fixtures do not share counters.
func (p *MemoryFixtureProvider) Clone() ports.FixtureProvider {
	p.mu.Lock()
	defer p.mu.Unlock()
	out := NewMemoryFixtureProvider()
	for k, v := range p.exactFixtures {
		out.exactFixtures[k] = v
	}
	for k, v := range p.seqFixtures {
		out.seqFixtures[k] = append([]api.Fixture(nil), v...)
	}
	return out
}

// HasOrderedFixtures reports whether any fixture uses ordered-sequence matching.
func HasOrderedFixtures(fixtures []api.Fixture) bool {
	for _, fx := range fixtures {
		if fx.MatchStrategy == api.MatchStrategyOrderedSequence {
			return true
		}
	}
	return false
}
