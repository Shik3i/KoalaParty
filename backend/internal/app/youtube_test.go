package app

import (
	"strings"
	"testing"
)

func TestFallbackTitle(t *testing.T) {
	long := strings.Repeat("a", 250)
	cases := []struct {
		name    string
		client  string
		videoID string
		want    string
	}{
		{"uses client title", "Client Title", "abc12345678", "Client Title"},
		{"trims whitespace", "  Spaced  ", "abc12345678", "Spaced"},
		{"blank becomes placeholder", "   ", "abc12345678", "YouTube video abc12345678"},
		{"caps at 200", long, "abc12345678", long[:200]},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := fallbackTitle(tc.client, tc.videoID); got != tc.want {
				t.Fatalf("fallbackTitle() = %q, want %q", got, tc.want)
			}
		})
	}
}
