// [fork] Custom pairing text resolver — user-configurable templates with i18n fallback.
//
// Wire points in commands_pairing.go (3 one-line call-sites, each tagged `// [fork] custom pairing text`):
//   - sendPairingReply       → c.resolvePairingDMText(code)
//   - sendGroupPairingReply  → c.resolvePairingGroupText(code)
//   - SendPairingApproved    → c.resolvePairingApprovedText(botName)
//
// Upstream-safety: the original pairing handlers are touched in exactly 3 lines;
// all new logic lives here so merge conflicts stay minimal when upstream evolves.
//
// The package-level helpers (renderPairingTemplate, normalizePairingLocale,
// composePairingCodeText, composePairingApprovedText) are pure functions used by
// the *Channel method wrappers and exercised directly in pairing_text_test.go.

package telegram

import (
	"strings"

	"github.com/nextlevelbuilder/goclaw/internal/i18n"
)

// renderPairingTemplate substitutes {code} and {bot_name} placeholders.
// An empty template returns "" so the caller can decide whether to fall back.
func renderPairingTemplate(tpl, code, botName string) string {
	if tpl == "" {
		return ""
	}
	out := strings.ReplaceAll(tpl, "{code}", code)
	out = strings.ReplaceAll(out, "{bot_name}", botName)
	return out
}

// normalizePairingLocale returns a catalog-supported locale, defaulting to English.
func normalizePairingLocale(locale string) string {
	switch locale {
	case i18n.LocaleEN, i18n.LocaleVI, i18n.LocaleZH:
		return locale
	}
	return i18n.LocaleEN
}

// composePairingCodeText handles the DM/Group templates which carry both
// {code} and {bot_name}. The i18n default is a 2-arg "%s ... %s" string.
func composePairingCodeText(custom, code, botName, locale, i18nKey string) string {
	if custom != "" {
		return renderPairingTemplate(custom, code, botName)
	}
	return i18n.T(normalizePairingLocale(locale), i18nKey, code, botName)
}

// composePairingApprovedText handles the Approved template which carries only
// {bot_name}. The i18n default is a 1-arg "%s" string.
func composePairingApprovedText(custom, botName, locale string) string {
	if custom != "" {
		return renderPairingTemplate(custom, "", botName)
	}
	return i18n.T(normalizePairingLocale(locale), i18n.MsgTelegramPairingApproved, botName)
}

// pairingLocale returns the configured fallback locale, normalized.
func (c *Channel) pairingLocale() string {
	return normalizePairingLocale(c.config.PairingLocale)
}

// botDisplayName returns the bot's Telegram @username (cached by telego after
// the first GetMe call in Start()) or a safe fallback.
func (c *Channel) botDisplayName() string {
	if name := c.bot.Username(); name != "" {
		return name
	}
	return "GoClaw"
}

// resolvePairingDMText returns the DM pairing reply text.
// Precedence: custom template (if set) → localized i18n default.
func (c *Channel) resolvePairingDMText(code string) string {
	return composePairingCodeText(
		c.config.PairingDMText, code, c.botDisplayName(),
		c.config.PairingLocale, i18n.MsgTelegramPairingDM,
	)
}

// resolvePairingGroupText returns the group pairing reply text.
// Precedence: custom template (if set) → localized i18n default.
func (c *Channel) resolvePairingGroupText(code string) string {
	return composePairingCodeText(
		c.config.PairingGroupText, code, c.botDisplayName(),
		c.config.PairingLocale, i18n.MsgTelegramPairingGroup,
	)
}

// resolvePairingApprovedText returns the post-approval notification text.
// botName is supplied by the approval flow; when empty, the cached bot
// username is used instead (matches the "GoClaw" fallback upstream had at
// commands_pairing.go:109-111).
func (c *Channel) resolvePairingApprovedText(botName string) string {
	if botName == "" {
		botName = c.botDisplayName()
	}
	return composePairingApprovedText(c.config.PairingApprovedText, botName, c.config.PairingLocale)
}
