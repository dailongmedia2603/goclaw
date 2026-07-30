//go:build spike

// Command fbcloak-spike is a throwaway POC for Phase 0 of the fbcloak plan.
// It is excluded from the default build via the `spike` build tag.
//
// Goal: verify feasibility of the four critical questions before committing
// to the full implementation:
//
//  1. Can we inject FB session cookies and pass /me without checkpoint?
//  2. Does inline stealth survive sannysoft + creepjs?
//  3. Can we open a thread directly via URL without scanning the inbox?
//  4. Does a 30-minute idle session attract checkpoints/captchas?
//
// Build:
//
//	go build -tags spike -o ./bin/fbcloak-spike ./cmd/fbcloak-spike/
//
// Run:
//
//	./bin/fbcloak-spike \
//	    --cookies-file ./cookies.json \
//	    --fanpage-id <PAGE_ID> \
//	    --conversation-id <t_xxxx> \
//	    --proxy-url socks5://user:pass@host:port \
//	    --headless=false
//
// Output: screenshots in ./docs/research/assets/fbcloak-spike/ and a JSON
// summary on stdout. The user fills the report template at
// ./docs/research/fbcloak-spike-report-2026.md and decides GREEN/YELLOW/RED.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"
)

func main() {
	cookiesFile := flag.String("cookies-file", "", "path to cookies JSON exported from a logged-in admin session (required)")
	fanpageID := flag.String("fanpage-id", "", "numeric Facebook page ID (required)")
	conversationID := flag.String("conversation-id", "", "thread ID like t_xxxxxxxx for direct-URL probe (S0.4)")
	pageUsername := flag.String("page-username", "", "page vanity name for Pages classic URL probe (optional)")
	proxyURL := flag.String("proxy-url", "", "socks5://user:pass@host:port (optional)")
	userAgent := flag.String("user-agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0.0.0 Safari/537.36", "User-Agent override")
	headless := flag.Bool("headless", false, "run Chrome headless (default: false for visual debug)")
	idleMinutes := flag.Int("idle-minutes", 30, "minutes to keep session idle for stress test (S0.5)")
	skipIdle := flag.Bool("skip-idle", false, "skip the 30-minute idle stress test")
	assetsDir := flag.String("assets-dir", "./docs/research/assets/fbcloak-spike", "directory to save screenshots + HAR")
	flag.Parse()

	if *cookiesFile == "" || *fanpageID == "" {
		fmt.Fprintln(os.Stderr, "usage: fbcloak-spike --cookies-file <file> --fanpage-id <id> [flags]")
		flag.PrintDefaults()
		os.Exit(2)
	}

	if err := os.MkdirAll(*assetsDir, 0o750); err != nil {
		log.Fatalf("mkdir assets dir: %v", err)
	}

	cookies, err := loadCookies(*cookiesFile)
	if err != nil {
		log.Fatalf("load cookies: %v", err)
	}
	if len(cookies) == 0 {
		log.Fatal("cookies file is empty")
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	cfg := runnerConfig{
		Cookies:        cookies,
		FanpageID:      *fanpageID,
		ConversationID: *conversationID,
		PageUsername:   *pageUsername,
		ProxyURL:       *proxyURL,
		UserAgent:      *userAgent,
		Headless:       *headless,
		IdleMinutes:    *idleMinutes,
		SkipIdle:       *skipIdle,
		AssetsDir:      *assetsDir,
	}

	r, err := newRunner(ctx, cfg)
	if err != nil {
		log.Fatalf("new runner: %v", err)
	}
	defer r.Close()

	report := SpikeReport{
		StartedAt: time.Now().UTC().Format(time.RFC3339),
		Config: ReportConfig{
			FanpageID:      *fanpageID,
			ConversationID: *conversationID,
			ProxyConfigured: *proxyURL != "",
			Headless:       *headless,
			IdleMinutes:    *idleMinutes,
		},
	}

	// S0.2 — Cookie inject + health probe
	report.CookieInject = r.RunCookieInject(ctx)

	// S0.3 — Stealth probes
	report.Sannysoft = r.RunSannysoft(ctx)
	report.Creepjs = r.RunCreepjs(ctx)

	// S0.4 — Direct thread URL (PRIMARY decision)
	if *conversationID != "" {
		report.DirectThreadURL = r.RunDirectThreadURL(ctx)
	}

	// S0.4b — Inbox listitem fallback (only if S0.4 fully fails)
	if *conversationID == "" || allDirectFailed(report.DirectThreadURL) {
		report.InboxScanner = r.RunInboxScannerProbe(ctx)
	}

	// S0.5 — Stress idle session
	if !*skipIdle {
		report.IdleStress = r.RunIdleStress(ctx)
	}

	report.FinishedAt = time.Now().UTC().Format(time.RFC3339)
	report.Decision = decide(report)

	out, _ := json.MarshalIndent(report, "", "  ")
	fmt.Println(string(out))

	summaryPath := filepath.Join(*assetsDir, "summary.json")
	if err := os.WriteFile(summaryPath, out, 0o640); err != nil {
		log.Printf("warn: write summary: %v", err)
	}
	fmt.Fprintln(os.Stderr, "summary saved to:", summaryPath)
}

// allDirectFailed reports true when every direct-URL pattern failed (so we
// should fall back to inbox scanner probe for the same spike run).
func allDirectFailed(d *DirectThreadURLResult) bool {
	if d == nil {
		return true
	}
	if d.BusinessSuiteOK || d.MessengerWebOK || d.PagesClassicOK {
		return false
	}
	return true
}

// decide returns GREEN, YELLOW, or RED based on the simple rule from the plan:
// ≥3 of 4 critical checks pass → GREEN.
func decide(r SpikeReport) string {
	pass := 0
	total := 4
	if r.CookieInject != nil && r.CookieInject.LoggedIn {
		pass++
	}
	if r.Sannysoft != nil && r.Sannysoft.Captured && r.Creepjs != nil && r.Creepjs.Captured {
		// Stealth is "pass" by capture only — visual review by user makes the call.
		pass++
	}
	if r.DirectThreadURL != nil && (r.DirectThreadURL.BusinessSuiteOK || r.DirectThreadURL.MessengerWebOK) {
		pass++
	}
	if r.IdleStress != nil && r.IdleStress.CheckpointHits == 0 {
		pass++
	}
	_ = total // kept for documentation
	switch {
	case pass >= 3:
		return "GREEN"
	case pass == 2:
		return "YELLOW"
	default:
		return "RED"
	}
}
