package facebookmessenger

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/nextlevelbuilder/goclaw/internal/channels"
)

const pairingDebounce = 60 * time.Second

// checkDMPolicy enforces DM policy for an incoming message.
// Returns true if the message should be processed, false if it was rejected or queued for pairing.
func (c *Channel) checkDMPolicy(ctx context.Context, senderID, chatID string) bool {
	result := c.CheckDMPolicy(ctx, senderID, c.cfg.DMPolicy)
	switch result {
	case channels.PolicyAllow:
		return true
	case channels.PolicyNeedsPairing:
		c.sendPairingReply(ctx, senderID, chatID, false)
		return false
	default:
		slog.Debug("facebook_personal: DM rejected by policy",
			"sender_id", senderID, "policy", c.cfg.DMPolicy)
		return false
	}
}

// checkGroupPolicy enforces group access policy.
func (c *Channel) checkGroupPolicy(ctx context.Context, senderID, chatID string) bool {
	result := c.CheckGroupPolicy(ctx, senderID, chatID, c.cfg.GroupPolicy)
	switch result {
	case channels.PolicyAllow:
		return true
	case channels.PolicyNeedsPairing:
		groupSenderID := fmt.Sprintf("group:%s", chatID)
		c.sendPairingReply(ctx, groupSenderID, chatID, true)
		return false
	default:
		slog.Debug("facebook_personal: group message rejected by policy",
			"group_id", chatID, "policy", c.cfg.GroupPolicy)
		return false
	}
}

// sendPairingReply creates a pairing record and sends a short prompt to the sender.
// isGroup is true when the pairing target is a group chat.
func (c *Channel) sendPairingReply(ctx context.Context, senderID, chatID string, isGroup bool) {
	ps := c.PairingService()
	if ps == nil {
		return
	}

	if !c.CanSendPairingNotif(senderID, pairingDebounce) {
		return
	}

	code, err := ps.RequestPairing(ctx, senderID, c.Name(), chatID, "default", nil)
	if err != nil {
		slog.Debug("facebook_personal: pairing request failed",
			"sender_id", senderID, "error", err)
		return
	}

	c.MarkPairingNotifSent(senderID)
	slog.Info("facebook_personal: pairing request created (silent — no outbound FB message)",
		"sender_id", senderID, "chat_id", chatID, "code", code, "is_group", isGroup)
	// Intentionally DO NOT send any message to the FB sender to avoid spam /
	// ban risk. Admin approves the pairing from the Nodes UI without the
	// sender being notified.
	_ = fmt.Sprint // keep fmt import used elsewhere in the file
}
