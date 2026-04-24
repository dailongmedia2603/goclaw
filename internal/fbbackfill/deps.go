package fbbackfill

import (
	"github.com/nextlevelbuilder/goclaw/internal/store"
)

// Deps bundles the dependencies needed to start the backfill subsystem.
// Populated in cmd/gateway.go and passed to Register.
//
// Defined here (no build tag) so the struct shape is identical across
// PG and sqliteonly builds — the caller (cmd/gateway.go) does not need
// build-tagged code. Register() is the build-tag-gated entry point.
type Deps struct {
	// Channel instance store — required. Used to read credentials + config
	// and to persist BackfillState into channel_instances.config._backfill.
	Instances store.ChannelInstanceStore

	// Episodic memory store — required. Summaries are written here.
	EpisodicStore store.EpisodicStore

	// LLM resolver — optional. Resolves the tenant's background provider
	// for long-conversation summarization. When nil, fallback concat path
	// is used for every conversation.
	LLMResolver LLMResolver

	// ClientFactory — optional. When nil, the default live Graph API
	// factory is used. Injected in tests to redirect to a mock server.
	ClientFactory ClientFactory

	// RegisterRPC — optional. When non-nil, RPC handlers are registered
	// with the gateway's MethodRouter. Signature lets the caller adapt
	// between the fbbackfill handler shape and the gateway.MethodHandler
	// shape without this package importing gateway.
	RegisterRPC func(method string, handler HandlerFunc)

	// Broadcast — optional. When non-nil, lifecycle events are pushed
	// to connected WS clients filtered by tenant. When nil, progress
	// events are discarded (state is still persisted and observable via
	// fb_backfill.status RPC).
	Broadcast BroadcastFunc

	// Tunables.
	MaxConcurrentJobs int
	SummarizerConfig  SummarizerConfig
}
