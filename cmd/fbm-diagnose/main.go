// fbm-diagnose — post-install health check for the FBM channel bundle.
//
// Runs on the recipient host after install-fbm-bundle.sh completes.
// Checks:
//   - sidecar /healthz reachable with Bearer token
//   - gateway /healthz reachable
//   - install marker file present + well-formed
//   - fork image tags match marker
//
// Exit codes:
//
//	0 — all green
//	1 — warnings (degraded state; feature works but something to fix)
//	2 — errors (feature not working)
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"time"
)

type marker struct {
	BundleVersion    string `json:"bundle_version"`
	InstallDate      string `json:"install_date"`
	UpstreamVersion  string `json:"goclaw_upstream_version"`
	SidecarPort      string `json:"sidecar_port"`
	InstanceName     string `json:"instance_name"`
	ComposeBackupBak string `json:"compose_backup_path"`
}

type report struct {
	BundleVersion    string           `json:"bundle_version"`
	InstallDate      string           `json:"install_date"`
	Sidecar          string           `json:"sidecar"`  // "healthy" | "down" | "unauthorized"
	Gateway          string           `json:"gateway"`  // "healthy" | "unreachable"
	Metrics          map[string]int64 `json:"metrics,omitempty"`
	Warnings         []string         `json:"warnings"`
	Errors           []string         `json:"errors"`
	ImagesPresent    map[string]bool  `json:"images_present"`
}

func main() {
	var (
		markerPath  = flag.String("marker", "/opt/goclaw/.fbm-bundle-installed", "path to install marker JSON")
		sidecarURL  = flag.String("sidecar-url", "http://localhost:29320", "sidecar base URL")
		gatewayURL  = flag.String("gateway-url", "http://localhost:18790", "gateway base URL")
		envFile     = flag.String("env", "/opt/goclaw/.env.fbm", "path to .env.fbm for FBM_AUTH_TOKEN")
		jsonOutput  = flag.Bool("json", false, "emit JSON report instead of human text")
		timeoutSec  = flag.Int("timeout", 5, "per-request timeout in seconds")
	)
	flag.Parse()

	r := &report{
		Warnings:      []string{},
		Errors:        []string{},
		ImagesPresent: map[string]bool{},
	}
	client := &http.Client{Timeout: time.Duration(*timeoutSec) * time.Second}

	// 1. Marker
	m, err := readMarker(*markerPath)
	if err != nil {
		r.Errors = append(r.Errors, fmt.Sprintf("marker file unreadable: %v", err))
	} else {
		r.BundleVersion = m.BundleVersion
		r.InstallDate = m.InstallDate
	}

	// 2. Secrets
	token := readAuthToken(*envFile)
	if token == "" {
		r.Warnings = append(r.Warnings, fmt.Sprintf("FBM_AUTH_TOKEN missing in %s — cannot probe sidecar", *envFile))
	}

	// 3. Sidecar health
	r.Sidecar = pingSidecar(client, *sidecarURL, token)

	// 4. Gateway health
	r.Gateway = pingGateway(client, *gatewayURL)

	// 5. Image presence (docker CLI)
	if m != nil && m.BundleVersion != "" {
		for _, img := range []string{"goclaw-fork", "goclaw-web-fork", "fbm-sidecar"} {
			tag := fmt.Sprintf("%s:%s", img, m.BundleVersion)
			present := dockerImageExists(tag)
			r.ImagesPresent[tag] = present
			if !present {
				r.Errors = append(r.Errors, fmt.Sprintf("missing docker image: %s", tag))
			}
		}
	}

	// Emit
	if *jsonOutput {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		_ = enc.Encode(r)
	} else {
		printHuman(r, m)
	}

	// Exit code
	switch {
	case len(r.Errors) > 0:
		os.Exit(2)
	case len(r.Warnings) > 0:
		os.Exit(1)
	default:
		os.Exit(0)
	}
}

func readMarker(path string) (*marker, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var m marker
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, err
	}
	return &m, nil
}

func readAuthToken(envFile string) string {
	f, err := os.Open(envFile)
	if err != nil {
		return ""
	}
	defer f.Close()
	data, _ := io.ReadAll(f)
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "FBM_AUTH_TOKEN=") {
			return strings.TrimSpace(strings.TrimPrefix(line, "FBM_AUTH_TOKEN="))
		}
	}
	return ""
}

func pingSidecar(client *http.Client, base, token string) string {
	req, err := http.NewRequest("GET", base+"/healthz", nil)
	if err != nil {
		return "down"
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := client.Do(req)
	if err != nil {
		return "down"
	}
	defer resp.Body.Close()
	switch resp.StatusCode {
	case http.StatusOK:
		return "healthy"
	case http.StatusUnauthorized:
		return "unauthorized"
	default:
		return fmt.Sprintf("bad_status_%d", resp.StatusCode)
	}
}

func pingGateway(client *http.Client, base string) string {
	resp, err := client.Get(base + "/healthz")
	if err != nil {
		return "unreachable"
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, resp.Body)
	if resp.StatusCode == http.StatusOK {
		return "healthy"
	}
	return fmt.Sprintf("bad_status_%d", resp.StatusCode)
}

func dockerImageExists(tag string) bool {
	cmd := exec.Command("docker", "image", "inspect", tag)
	cmd.Stdout = io.Discard
	cmd.Stderr = io.Discard
	return cmd.Run() == nil
}

func printHuman(r *report, m *marker) {
	fmt.Printf("FBM Diagnose Report\n")
	fmt.Printf("===================\n")
	if m != nil {
		fmt.Printf("Bundle version:   %s\n", m.BundleVersion)
		fmt.Printf("Installed:        %s\n", m.InstallDate)
		fmt.Printf("Upstream:         %s\n", m.UpstreamVersion)
	}
	fmt.Printf("Sidecar:          %s\n", statusIcon(r.Sidecar))
	fmt.Printf("Gateway:          %s\n", statusIcon(r.Gateway))
	fmt.Printf("\nImages:\n")
	for tag, present := range r.ImagesPresent {
		mark := "✓"
		if !present {
			mark = "✗"
		}
		fmt.Printf("  [%s] %s\n", mark, tag)
	}
	if len(r.Warnings) > 0 {
		fmt.Printf("\nWarnings:\n")
		for _, w := range r.Warnings {
			fmt.Printf("  ⚠  %s\n", w)
		}
	}
	if len(r.Errors) > 0 {
		fmt.Printf("\nErrors:\n")
		for _, e := range r.Errors {
			fmt.Printf("  ✗  %s\n", e)
		}
	}
}

func statusIcon(s string) string {
	switch s {
	case "healthy":
		return "✓ healthy"
	case "unreachable", "down":
		return "✗ " + s
	default:
		return "⚠  " + s
	}
}
