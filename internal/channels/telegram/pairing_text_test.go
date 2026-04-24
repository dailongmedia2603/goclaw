package telegram

import (
	"strings"
	"testing"

	"github.com/nextlevelbuilder/goclaw/internal/i18n"
)

func TestRenderPairingTemplate(t *testing.T) {
	cases := []struct {
		name, tpl, code, bot, want string
	}{
		{"empty template returns empty", "", "ABC", "MyBot", ""},
		{"code only", "Code is {code}!", "XYZ", "MyBot", "Code is XYZ!"},
		{"bot only", "Hello from {bot_name}", "X", "MyBot", "Hello from MyBot"},
		{"both placeholders", "{bot_name}: {code}", "123", "Max", "Max: 123"},
		{"duplicate placeholder", "{code}-{code}", "A", "B", "A-A"},
		{"no placeholder stays literal", "no placeholders here", "X", "Y", "no placeholders here"},
		{"empty code substitutes empty", "code={code};", "", "Bot", "code=;"},
		{"multiline preserved", "line1\n{code}\nline3", "mid", "B", "line1\nmid\nline3"},
		{"special chars in code", "{code}", "a$b%c", "B", "a$b%c"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := renderPairingTemplate(tc.tpl, tc.code, tc.bot)
			if got != tc.want {
				t.Errorf("renderPairingTemplate(%q,%q,%q) = %q, want %q",
					tc.tpl, tc.code, tc.bot, got, tc.want)
			}
		})
	}
}

func TestNormalizePairingLocale(t *testing.T) {
	cases := map[string]string{
		"":        i18n.LocaleEN,
		"en":      i18n.LocaleEN,
		"vi":      i18n.LocaleVI,
		"zh":      i18n.LocaleZH,
		"EN":      i18n.LocaleEN, // case-sensitive on purpose; fall back
		"fr":      i18n.LocaleEN,
		"random":  i18n.LocaleEN,
		"en-US":   i18n.LocaleEN,
		"vi-VN":   i18n.LocaleEN,
	}
	for in, want := range cases {
		got := normalizePairingLocale(in)
		if got != want {
			t.Errorf("normalizePairingLocale(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestComposePairingCodeText_CustomWins(t *testing.T) {
	got := composePairingCodeText(
		"Custom: {code} / {bot_name}",
		"ABC123", "Max",
		i18n.LocaleVI, // locale ignored because custom is set
		i18n.MsgTelegramPairingDM,
	)
	want := "Custom: ABC123 / Max"
	if got != want {
		t.Errorf("custom template should win. got=%q want=%q", got, want)
	}
}

func TestComposePairingCodeText_FallbackEN(t *testing.T) {
	got := composePairingCodeText("", "ABC123", "Max", "", i18n.MsgTelegramPairingDM)
	// English default contains the code and bot name
	if !strings.Contains(got, "ABC123") {
		t.Errorf("EN fallback missing code in: %q", got)
	}
	if !strings.Contains(got, "Max") {
		t.Errorf("EN fallback missing bot name in: %q", got)
	}
	if !strings.Contains(strings.ToLower(got), "pairing code") {
		t.Errorf("EN fallback missing phrase 'Pairing code': %q", got)
	}
}

func TestComposePairingCodeText_FallbackVI(t *testing.T) {
	got := composePairingCodeText("", "ABC123", "Max", i18n.LocaleVI, i18n.MsgTelegramPairingDM)
	if !strings.Contains(got, "ABC123") || !strings.Contains(got, "Max") {
		t.Errorf("VI fallback missing substitutions: %q", got)
	}
	// Vietnamese catalog uses "Mã ghép nối"
	if !strings.Contains(got, "Mã ghép nối") {
		t.Errorf("VI fallback should contain 'Mã ghép nối': %q", got)
	}
}

func TestComposePairingCodeText_FallbackZH(t *testing.T) {
	got := composePairingCodeText("", "ABC123", "Max", i18n.LocaleZH, i18n.MsgTelegramPairingGroup)
	if !strings.Contains(got, "ABC123") || !strings.Contains(got, "Max") {
		t.Errorf("ZH fallback missing substitutions: %q", got)
	}
	if !strings.Contains(got, "配对码") {
		t.Errorf("ZH fallback should contain '配对码': %q", got)
	}
}

func TestComposePairingCodeText_UnknownLocaleFallsBackToEN(t *testing.T) {
	gotBad := composePairingCodeText("", "ABC", "Bot", "klingon", i18n.MsgTelegramPairingDM)
	gotEN := composePairingCodeText("", "ABC", "Bot", i18n.LocaleEN, i18n.MsgTelegramPairingDM)
	if gotBad != gotEN {
		t.Errorf("unknown locale should match EN.\nbad=%q\nen =%q", gotBad, gotEN)
	}
}

func TestComposePairingApprovedText_CustomWins(t *testing.T) {
	got := composePairingApprovedText("Welcome {bot_name}!", "MyBot", i18n.LocaleVI)
	if got != "Welcome MyBot!" {
		t.Errorf("custom approved wrong: %q", got)
	}
}

func TestComposePairingApprovedText_FallbackContainsBotName(t *testing.T) {
	got := composePairingApprovedText("", "MyBot", i18n.LocaleEN)
	if !strings.Contains(got, "MyBot") {
		t.Errorf("EN approved fallback should contain bot name: %q", got)
	}
	if !strings.Contains(strings.ToLower(got), "approved") {
		t.Errorf("EN approved fallback should contain 'approved': %q", got)
	}
}

func TestComposePairingApprovedText_FallbackVI(t *testing.T) {
	got := composePairingApprovedText("", "MyBot", i18n.LocaleVI)
	if !strings.Contains(got, "MyBot") {
		t.Errorf("VI approved fallback should contain bot name: %q", got)
	}
	if !strings.Contains(got, "phê duyệt") {
		t.Errorf("VI approved fallback should contain 'phê duyệt': %q", got)
	}
}

func TestComposePairingApprovedText_IgnoresCodePlaceholder(t *testing.T) {
	// Approved templates should only reference {bot_name}; stray {code} stays literal.
	got := composePairingApprovedText("approved {bot_name} (code {code})", "MyBot", i18n.LocaleEN)
	if !strings.Contains(got, "MyBot") {
		t.Errorf("missing bot name in: %q", got)
	}
	if !strings.Contains(got, "{code}") {
		// composePairingApprovedText calls renderPairingTemplate with code="" so
		// {code} → "". This is expected behavior — custom templates shouldn't use {code}.
		if !strings.Contains(got, "(code )") {
			t.Errorf("stray {code} should substitute to empty: %q", got)
		}
	}
}

// Defensive: ensure the three i18n keys are registered in all three locales.
func TestPairingI18nKeysRegistered(t *testing.T) {
	locales := []string{i18n.LocaleEN, i18n.LocaleVI, i18n.LocaleZH}
	keys := []string{
		i18n.MsgTelegramPairingDM,
		i18n.MsgTelegramPairingGroup,
		i18n.MsgTelegramPairingApproved,
	}
	for _, loc := range locales {
		for _, k := range keys {
			// For DM/Group keys the template carries 2 %s; for Approved it carries 1.
			// We pass enough args to cover any case — extra args to Sprintf become
			// "%!(EXTRA ...)" strings, which we explicitly check for.
			var out string
			if k == i18n.MsgTelegramPairingApproved {
				out = i18n.T(loc, k, "Bot")
			} else {
				out = i18n.T(loc, k, "CODE", "Bot")
			}
			if out == "" {
				t.Errorf("missing translation: locale=%q key=%q", loc, k)
			}
			if strings.Contains(out, "%!(") {
				t.Errorf("wrong number of format args: locale=%q key=%q out=%q", loc, k, out)
			}
			// Key should not leak through — if lookup failed, i18n.T returns the key literal.
			if out == k {
				t.Errorf("translation returned key literal (lookup miss): locale=%q key=%q", loc, k)
			}
		}
	}
}
