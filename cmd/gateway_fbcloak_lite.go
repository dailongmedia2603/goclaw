//go:build sqliteonly

package cmd

import (
	"github.com/nextlevelbuilder/goclaw/internal/config"
	"github.com/nextlevelbuilder/goclaw/internal/eventbus"
	"github.com/nextlevelbuilder/goclaw/internal/gateway"
	"github.com/nextlevelbuilder/goclaw/internal/store"
)

// wireFBCloak no-op stub for sqliteonly (Lite desktop) builds. The real
// implementation is in gateway_fbcloak.go behind !sqliteonly.
func wireFBCloak(_ *gateway.Server, _ *store.Stores, _ *config.Config, _ eventbus.DomainEventBus) {
}
