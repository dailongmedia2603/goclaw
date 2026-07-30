package fbbackfill

import (
	"context"

	"github.com/google/uuid"
	"github.com/nextlevelbuilder/goclaw/internal/providers"
	"github.com/nextlevelbuilder/goclaw/internal/providerresolve"
	"github.com/nextlevelbuilder/goclaw/internal/store"
)

// providerRegistryResolver is an LLMResolver backed by the shared
// providerresolve.ResolveBackgroundProvider helper. This is the resolver
// used in production — tests use fakeResolver from summarizer_test.go.
type providerRegistryResolver struct {
	registry      *providers.Registry
	systemConfigs store.SystemConfigStore
}

// NewProviderRegistryResolver builds an LLMResolver backed by the tenant's
// background provider configuration. Safe to pass a nil registry or store
// — the resolver will return (nil, "") which triggers the summarizer's
// concat-path fallback.
func NewProviderRegistryResolver(registry *providers.Registry, systemConfigs store.SystemConfigStore) LLMResolver {
	return &providerRegistryResolver{registry: registry, systemConfigs: systemConfigs}
}

func (r *providerRegistryResolver) Resolve(ctx context.Context, tenantID uuid.UUID) (LLMClient, string) {
	if r.registry == nil {
		return nil, ""
	}
	p, model := providerresolve.ResolveBackgroundProvider(ctx, tenantID, r.registry, r.systemConfigs)
	if p == nil {
		return nil, ""
	}
	// providers.Provider structurally satisfies LLMClient (Chat + DefaultModel).
	return p, model
}
