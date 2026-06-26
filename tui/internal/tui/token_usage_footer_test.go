package tui

import (
	"strings"
	"testing"

	"github.com/anthropics/lingtai-tui/internal/fs"
)

// The token-usage footer condenses an llm_response event's per-round token
// scalars into a single compact line rendered at the bottom of the ctrl+o API
// call group: input, cache miss, output, cache rate. Only these four derived
// numbers are shown — never the noisy `_meta` envelope hidden by PR #440.
func TestFormatTokenUsageFooter(t *testing.T) {
	tests := []struct {
		name  string
		usage *fs.TokenUsage
		// substrings that must appear
		want []string
		// substrings that must NOT appear
		notWant []string
		// when set, the whole footer must equal this exactly
		exact string
	}{
		{
			name:  "nil usage — older event with no token fields",
			usage: nil,
			exact: "",
		},
		{
			name:  "all zero — nothing usable",
			usage: &fs.TokenUsage{},
			exact: "",
		},
		{
			name: "full round humanized",
			usage: &fs.TokenUsage{
				Input:  181585,
				Output: 2275,
				Cached: 180224,
			},
			// input 181585; cache miss = 181585-180224 = 1361; output 2275;
			// cache rate = 180224/181585 = 99.3%
			want: []string{"181.6k", "1.4k", "2.3k", "99.3%"},
		},
		{
			name: "small counts not abbreviated",
			usage: &fs.TokenUsage{
				Input:  100,
				Output: 40,
				Cached: 60,
			},
			// miss = 40, rate = 60%
			want: []string{"100", "40", "60.0%"},
		},
		{
			name: "no cache — rate 0, miss equals input",
			usage: &fs.TokenUsage{
				Input:  500,
				Output: 120,
				Cached: 0,
			},
			want: []string{"500", "120", "0.0%"},
		},
		{
			name: "estimated round is marked",
			usage: &fs.TokenUsage{
				Input:     1000,
				Output:    200,
				Cached:    800,
				Estimated: true,
			},
			want: []string{"1.0k", "200", "80.0%", "~"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatTokenUsageFooter(tt.usage)
			if tt.exact != "" || tt.usage == nil || (tt.usage != nil && tt.usage.Input == 0 && tt.usage.Output == 0 && tt.usage.Cached == 0) {
				if got != tt.exact {
					t.Fatalf("got %q, want exact %q", got, tt.exact)
				}
				return
			}
			for _, w := range tt.want {
				if !strings.Contains(got, w) {
					t.Errorf("footer %q missing %q", got, w)
				}
			}
			for _, nw := range tt.notWant {
				if strings.Contains(got, nw) {
					t.Errorf("footer %q should not contain %q", got, nw)
				}
			}
		})
	}
}

func TestHumanizeTokenCount(t *testing.T) {
	cases := map[int64]string{
		0:       "0",
		42:      "42",
		999:     "999",
		1000:    "1.0k",
		1361:    "1.4k",
		181585:  "181.6k",
		2275:    "2.3k",
		1000000: "1.0M",
		2500000: "2.5M",
	}
	for in, want := range cases {
		if got := humanizeTokenCount(in); got != want {
			t.Errorf("humanizeTokenCount(%d) = %q, want %q", in, got, want)
		}
	}
}
