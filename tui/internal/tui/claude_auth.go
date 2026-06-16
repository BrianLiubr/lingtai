package tui

import (
	"context"
	"encoding/json"
	"os/exec"
	"strings"
	"time"
)

// claudeAuthStatusTimeout bounds how long the TUI waits on the Claude
// Code CLI before giving up. The detection runs at render/health-check
// time, so it must never hang the UI; a slow or wedged `claude` is
// treated as "not configured".
const claudeAuthStatusTimeout = 4 * time.Second

// claudeCodeAuthConfigured reports whether the local Claude Code CLI
// (`claude`) is installed and reports a logged-in session. The
// claude-agent-sdk preset authenticates through that existing CLI login —
// the TUI implements no separate Anthropic OAuth flow and stores no token
// of its own. We only ask the CLI for its status; we never read Claude's
// credential files and never print any secret.
//
// Returns false on every uncertain outcome: CLI missing, nonzero exit,
// timeout, or output that does not clearly say "logged in". Pass the
// result into preset.AuthState.ClaudeCodeAuthConfigured so the credential
// guard can judge claude-agent-sdk presets without importing this package.
func claudeCodeAuthConfigured() bool {
	if _, err := exec.LookPath("claude"); err != nil {
		return false
	}
	ctx, cancel := context.WithTimeout(context.Background(), claudeAuthStatusTimeout)
	defer cancel()
	// `--json` is the current default, but pass it explicitly so a future
	// default change can't flip us to text-only output unexpectedly.
	cmd := exec.CommandContext(ctx, "claude", "auth", "status", "--json")
	out, err := cmd.CombinedOutput()
	if ctx.Err() != nil {
		return false // timed out / cancelled
	}
	if err != nil {
		// Nonzero exit. Some CLI versions exit nonzero when logged out;
		// still try to parse in case the body is a usable JSON status, but
		// the tolerant parser returns false for anything not clearly
		// logged-in.
		return parseClaudeAuthStatus(out)
	}
	return parseClaudeAuthStatus(out)
}

// parseClaudeAuthStatus tolerantly decides whether `claude auth status`
// output indicates a logged-in session. It prefers the structured JSON
// "loggedIn" boolean (the CLI's default --json shape); if no JSON object
// is found it falls back to a conservative text-signal check. Anything
// ambiguous resolves to false.
func parseClaudeAuthStatus(out []byte) bool {
	s := strings.TrimSpace(string(out))
	if s == "" {
		return false
	}

	// Prefer JSON: find the first balanced-looking object and read
	// loggedIn. Status JSON may be preceded by log/progress lines, so we
	// scan from the first '{'.
	if i := strings.IndexByte(s, '{'); i >= 0 {
		if j := strings.LastIndexByte(s, '}'); j > i {
			var doc struct {
				LoggedIn *bool `json:"loggedIn"`
			}
			if err := json.Unmarshal([]byte(s[i:j+1]), &doc); err == nil && doc.LoggedIn != nil {
				return *doc.LoggedIn
			}
		}
	}

	// Text fallback. Treat an explicit not-logged-in signal as decisive
	// before looking for the positive phrase, so "Not logged in" can't be
	// matched by a naive "logged in" substring.
	lower := strings.ToLower(s)
	if strings.Contains(lower, "not logged in") || strings.Contains(lower, "logged out") {
		return false
	}
	return strings.Contains(lower, "logged in")
}
