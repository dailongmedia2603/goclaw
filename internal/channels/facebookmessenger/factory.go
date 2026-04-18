package facebookmessenger

import (
	"encoding/json"
	"fmt"

	"github.com/nextlevelbuilder/goclaw/internal/bus"
	"github.com/nextlevelbuilder/goclaw/internal/channels"
	"github.com/nextlevelbuilder/goclaw/internal/store"
)

// Factory satisfies channels.ChannelFactory.
// Compile-time assertion below catches upstream signature changes.
var _ channels.ChannelFactory = Factory

// Factory creates a facebook_personal Channel from DB instance data.
// Register in cmd/gateway.go with:
//
//	instanceLoader.RegisterFactory(channels.TypeFacebookPersonal, facebookmessenger.Factory)
func Factory(
	name string,
	creds json.RawMessage,
	cfg json.RawMessage,
	msgBus *bus.MessageBus,
	pairingSvc store.PairingStore,
) (channels.Channel, error) {
	c, err := parseCredentials(creds)
	if err != nil {
		return nil, fmt.Errorf("facebook_personal: parse credentials: %w", err)
	}
	config, err := parseConfig(cfg)
	if err != nil {
		return nil, fmt.Errorf("facebook_personal: parse config: %w", err)
	}
	ch := New(name, c, config, msgBus)
	if pairingSvc != nil {
		ch.SetPairingService(pairingSvc)
	}
	return ch, nil
}
