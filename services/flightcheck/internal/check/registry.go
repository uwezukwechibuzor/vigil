package check

import "context"

// Check is the interface every flight check must implement.
type Check interface {
	// Name returns the stable identifier for this check.
	Name() string
	// Run executes the check and returns a Result or an error.
	Run(ctx context.Context, req *Request, deps *Deps) (*Result, error)
}

// Registry holds an ordered list of checks to run.
type Registry struct{ checks []Check }

// NewRegistry creates an empty Registry.
func NewRegistry() *Registry { return &Registry{} }

// Register appends a check to the registry.
func (r *Registry) Register(c Check) { r.checks = append(r.checks, c) }

// All returns all registered checks in registration order.
func (r *Registry) All() []Check { return r.checks }
