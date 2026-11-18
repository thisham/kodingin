// Package tool defines what a tool is in the domain. Implementations live in infrastructure.
package tool

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
)

// Tool is a capability the agent can invoke.
type Tool interface {
	Name() string
	Description() string
	Schema() json.RawMessage // JSON Schema for arguments
	// Dangerous tools (writes, shell) require user approval.
	Dangerous() bool
	Run(ctx context.Context, args json.RawMessage) (string, error)
}

type Registry struct{ tools map[string]Tool }

func NewRegistry(ts ...Tool) *Registry {
	r := &Registry{tools: map[string]Tool{}}
	for _, t := range ts {
		r.tools[t.Name()] = t
	}
	return r
}

func (r *Registry) Get(name string) (Tool, error) {
	t, ok := r.tools[name]
	if !ok {
		return nil, fmt.Errorf("unknown tool %q", name)
	}
	return t, nil
}

func (r *Registry) All() []Tool {
	out := make([]Tool, 0, len(r.tools))
	for _, t := range r.tools {
		out = append(out, t)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name() < out[j].Name() })
	return out
}
