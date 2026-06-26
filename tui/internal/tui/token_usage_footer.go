package tui

import (
	"fmt"

	"github.com/anthropics/lingtai-tui/i18n"
	"github.com/anthropics/lingtai-tui/internal/fs"
)

// formatTokenUsageFooter condenses an llm_response event's per-round token
// scalars into a single compact line rendered at the bottom of the ctrl+o API
// call group:
//
//	tokens: input 181.6k · cache miss 1.4k · output 2.3k · cache rate 99.3%
//
// Only the four derived numbers are shown — never the noisy `_meta` envelope
// hidden by PR #440. cache miss reuses cacheMiss() (input - cached, clamped ≥0)
// and cache rate reuses formatCacheRate(), the same helpers the agent-detail
// token panel uses (props.go), so the wording stays consistent across the UI.
//
// Returns "" when usage is nil (older events with no token fields) or carries
// no usable scalar, so the caller writes no footer line. Estimated rounds (the
// kernel derived the count because the provider returned no usage) are marked
// with a leading "~".
func formatTokenUsageFooter(usage *fs.TokenUsage) string {
	if usage == nil {
		return ""
	}
	if usage.Input == 0 && usage.Output == 0 && usage.Cached == 0 {
		return ""
	}
	line := fmt.Sprintf(
		i18n.T("mail.token_usage_footer"),
		humanizeTokenCount(usage.Input),
		humanizeTokenCount(cacheMiss(usage.Cached, usage.Input)),
		humanizeTokenCount(usage.Output),
		formatCacheRate(usage.Cached, usage.Input),
	)
	if usage.Estimated {
		line = "~" + line
	}
	return line
}

// humanizeTokenCount renders a token count compactly: small counts verbatim,
// thousands as "1.4k", millions as "2.5M". One decimal place keeps the footer
// short while still distinguishing nearby magnitudes.
func humanizeTokenCount(n int64) string {
	if n < 0 {
		return "-" + humanizeTokenCount(-n)
	}
	switch {
	case n >= 1_000_000:
		return fmt.Sprintf("%.1fM", float64(n)/1_000_000)
	case n >= 1_000:
		return fmt.Sprintf("%.1fk", float64(n)/1_000)
	default:
		return fmt.Sprintf("%d", n)
	}
}
