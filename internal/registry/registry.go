package registry

import (
	"fmt"
	"sync"

	"gust/internal/ports"
)

// Registry manages thread-safe registration and discovery of pluggable components.
type Registry struct {
	mu          sync.RWMutex
	evaluators  map[string]ports.Evaluator
	mutators    map[string]ports.Mutator
	testRunners map[string]ports.TestRunner
}

// New creates an initialized empty Registry.
func New() *Registry {
	return &Registry{
		evaluators:  make(map[string]ports.Evaluator),
		mutators:    make(map[string]ports.Mutator),
		testRunners: make(map[string]ports.TestRunner),
	}
}

// DefaultRegistry is a global registry instance for built-ins.
var DefaultRegistry = New()

// Evaluators
func (r *Registry) RegisterEvaluator(e ports.Evaluator) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	name := e.Name()
	if _, exists := r.evaluators[name]; exists {
		return fmt.Errorf("evaluator %q already registered", name)
	}
	r.evaluators[name] = e
	return nil
}

// ReplaceEvaluator registers or overwrites an evaluator (used for judge plugins).
func (r *Registry) ReplaceEvaluator(e ports.Evaluator) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.evaluators[e.Name()] = e
}

func (r *Registry) GetEvaluator(name string) (ports.Evaluator, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	e, ok := r.evaluators[name]
	return e, ok
}

func (r *Registry) ListEvaluators() []ports.Evaluator {
	r.mu.RLock()
	defer r.mu.RUnlock()
	list := make([]ports.Evaluator, 0, len(r.evaluators))
	for _, e := range r.evaluators {
		list = append(list, e)
	}
	return list
}

// Mutators
func (r *Registry) RegisterMutator(m ports.Mutator) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	name := m.Name()
	if _, exists := r.mutators[name]; exists {
		return fmt.Errorf("mutator %q already registered", name)
	}
	r.mutators[name] = m
	return nil
}

func (r *Registry) GetMutator(name string) (ports.Mutator, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	m, ok := r.mutators[name]
	return m, ok
}

func (r *Registry) ListMutators() []ports.Mutator {
	r.mu.RLock()
	defer r.mu.RUnlock()
	list := make([]ports.Mutator, 0, len(r.mutators))
	for _, m := range r.mutators {
		list = append(list, m)
	}
	return list
}

// TestRunners
func (r *Registry) RegisterTestRunner(tr ports.TestRunner) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	name := tr.Name()
	if _, exists := r.testRunners[name]; exists {
		return fmt.Errorf("test runner %q already registered", name)
	}
	r.testRunners[name] = tr
	return nil
}

func (r *Registry) GetTestRunner(name string) (ports.TestRunner, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	tr, ok := r.testRunners[name]
	return tr, ok
}

func (r *Registry) ListTestRunners() []ports.TestRunner {
	r.mu.RLock()
	defer r.mu.RUnlock()
	list := make([]ports.TestRunner, 0, len(r.testRunners))
	for _, tr := range r.testRunners {
		list = append(list, tr)
	}
	return list
}
