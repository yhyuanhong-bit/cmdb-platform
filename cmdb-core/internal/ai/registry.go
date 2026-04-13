package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"sync"

	"github.com/google/uuid"
)

// ModelRecord describes a prediction_model row in a way that decouples the
// registry from the concrete sqlc-generated Queries type.  When the DB layer
// adds ListEnabledModels it should return rows convertible to this shape.
type ModelRecord struct {
	ID       uuid.UUID
	Name     string
	Provider string // "dify", "openai", "claude", "local_llm", "custom"
	Endpoint string
	Config   json.RawMessage
}

// ModelLister is the subset of the DB layer the registry needs.
// Implementations may wrap dbgen.Queries once the prediction_models table and
// ListEnabledModels query exist.
type ModelLister interface {
	ListEnabledModels(ctx context.Context) ([]ModelRecord, error)
}

// ProviderInfo is a read-only summary returned by Registry.List.
type ProviderInfo struct {
	Name string `json:"name"`
	Type string `json:"type"`
}

// Registry is a thread-safe store of named AIProvider instances.
type Registry struct {
	mu        sync.RWMutex
	providers map[string]AIProvider
}

// NewRegistry creates an empty provider registry.
func NewRegistry() *Registry {
	return &Registry{
		providers: make(map[string]AIProvider),
	}
}

// Register adds (or replaces) a provider in the registry.
func (r *Registry) Register(p AIProvider) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.providers[p.Name()] = p
}

// Get returns a provider by name.
func (r *Registry) Get(name string) (AIProvider, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	p, ok := r.providers[name]
	return p, ok
}

// List returns a snapshot of all registered providers.
func (r *Registry) List() []ProviderInfo {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]ProviderInfo, 0, len(r.providers))
	for _, p := range r.providers {
		out = append(out, ProviderInfo{Name: p.Name(), Type: p.Type()})
	}
	return out
}

// LoadFromDB reads enabled models from the database and registers
// corresponding providers.  If lister is nil (e.g. the migration hasn't
// landed yet) the call is a safe no-op.
func (r *Registry) LoadFromDB(ctx context.Context, lister ModelLister) error {
	if lister == nil {
		slog.Warn("ai.Registry.LoadFromDB: model lister is nil, skipping provider load")
		return nil
	}

	models, err := lister.ListEnabledModels(ctx)
	if err != nil {
		return fmt.Errorf("ai: list enabled models: %w", err)
	}

	for _, m := range models {
		p, err := providerFromRecord(m)
		if err != nil {
			slog.Warn("ai.Registry.LoadFromDB: skipping model",
				"name", m.Name, "provider", m.Provider, "err", err)
			continue
		}
		r.Register(p)
		slog.Info("ai.Registry.LoadFromDB: registered provider",
			"name", p.Name(), "type", p.Type())
	}
	return nil
}

// providerFromRecord constructs a concrete AIProvider from a ModelRecord.
func providerFromRecord(m ModelRecord) (AIProvider, error) {
	var cfg map[string]any
	if len(m.Config) > 0 {
		if err := json.Unmarshal(m.Config, &cfg); err != nil {
			return nil, fmt.Errorf("unmarshal config for %s: %w", m.Name, err)
		}
	}
	if cfg == nil {
		cfg = map[string]any{}
	}

	switch m.Provider {
	case "dify":
		apiKey := stringFromConfig(cfg, "api_key", "")
		workflowID := stringFromConfig(cfg, "workflow_id", "")
		return NewDifyProvider(m.Name, m.Endpoint, apiKey, workflowID), nil

	case "openai", "claude", "local_llm":
		apiKey := stringFromConfig(cfg, "api_key", "")
		model := stringFromConfig(cfg, "model", "gpt-4o")
		return NewLLMProvider(m.Name, m.Provider, m.Endpoint, apiKey, model), nil

	case "custom":
		return NewCustomProvider(m.Name, m.Endpoint), nil

	default:
		return nil, fmt.Errorf("unknown provider type %q", m.Provider)
	}
}

// stringFromConfig extracts a string value from a generic config map,
// returning fallback if the key is missing or not a string.
func stringFromConfig(config map[string]any, key, fallback string) string {
	v, ok := config[key]
	if !ok {
		return fallback
	}
	s, ok := v.(string)
	if !ok {
		return fallback
	}
	return s
}
