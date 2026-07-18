package app

import (
	"context"
	"strings"
	"testing"
)

func TestResolveMediaTitle(t *testing.T) {
	ctx := context.Background()
	long := strings.Repeat("a", 250)
	cases := []struct {
		name     string
		fetch    func(context.Context, string) string
		fallback string
		want     string
	}{
		{"fetched title wins", func(context.Context, string) string { return "Real Title" }, "YouTube video abc12345678", "Real Title"},
		{"empty fetch keeps fallback", func(context.Context, string) string { return "" }, "Client Title", "Client Title"},
		{"nil fetcher uses fallback", nil, "Client Title", "Client Title"},
		{"blank fallback becomes placeholder", nil, "   ", "YouTube video abc12345678"},
		{"title is capped at 200", func(context.Context, string) string { return long }, "", long[:200]},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			a := &application{fetchTitle: tc.fetch}
			if got := a.resolveMediaTitle(ctx, "abc12345678", tc.fallback); got != tc.want {
				t.Fatalf("resolveMediaTitle() = %q, want %q", got, tc.want)
			}
		})
	}
}
