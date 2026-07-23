package app

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestFetchSponsorSegmentsPrivacyAndFiltering(t *testing.T) {
	const videoID = "dQw4w9WgXcQ"
	sum := sha256.Sum256([]byte(videoID))
	wantPrefix := hex.EncodeToString(sum[:])[:4]

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// The full video ID must never appear in the request — only the hash prefix.
		if strings.Contains(r.URL.String(), videoID) {
			t.Errorf("request leaked the full video ID: %s", r.URL)
		}
		if !strings.HasSuffix(r.URL.Path, "/api/skipSegments/"+wantPrefix) {
			t.Errorf("unexpected path %q, want hash prefix %q", r.URL.Path, wantPrefix)
		}
		w.Header().Set("Content-Type", "application/json")
		// Two videos share this hash prefix; only ours must be returned, and a
		// non-skip / unknown-category / inverted segment must be dropped.
		_, _ = w.Write([]byte(`[
			{"videoID":"` + videoID + `","segments":[
				{"category":"sponsor","actionType":"skip","segment":[10.5,20.0]},
				{"category":"intro","actionType":"skip","segment":[0,5]},
				{"category":"highlight","actionType":"poi","segment":[30,30]},
				{"category":"sponsor","actionType":"mute","segment":[40,50]},
				{"category":"unknown","actionType":"skip","segment":[60,70]},
				{"category":"sponsor","actionType":"skip","segment":[90,80]}
			]},
			{"videoID":"someOther1","segments":[{"category":"sponsor","actionType":"skip","segment":[1,2]}]}
		]`))
	}))
	defer server.Close()
	old := sponsorAPIBase
	sponsorAPIBase = server.URL
	defer func() { sponsorAPIBase = old }()

	got := fetchSponsorSegments(context.Background(), videoID)
	if len(got) != 2 {
		t.Fatalf("expected 2 valid skip segments, got %d: %+v", len(got), got)
	}
	// Sorted by start: intro (0-5) then sponsor (10.5-20).
	if got[0].Category != "intro" || got[0].Start != 0 || got[0].End != 5 {
		t.Errorf("unexpected first segment: %+v", got[0])
	}
	if got[1].Category != "sponsor" || got[1].Start != 10.5 || got[1].End != 20 {
		t.Errorf("unexpected second segment: %+v", got[1])
	}
}

func TestFetchSponsorSegmentsNotFound(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()
	old := sponsorAPIBase
	sponsorAPIBase = server.URL
	defer func() { sponsorAPIBase = old }()

	if got := fetchSponsorSegments(context.Background(), "novideo0000"); got != nil {
		t.Fatalf("expected nil for 404, got %+v", got)
	}
}

func TestSegmentCacheMemoizesAndExpires(t *testing.T) {
	var calls int32
	now := time.Unix(0, 0)
	cache := newSegmentCache(func(_ context.Context, videoID string) []sponsorSegment {
		atomic.AddInt32(&calls, 1)
		return []sponsorSegment{{Start: 1, End: 2, Category: "sponsor"}}
	})
	cache.now = func() time.Time { return now }

	cache.get(context.Background(), "vid")
	cache.get(context.Background(), "vid")
	if got := atomic.LoadInt32(&calls); got != 1 {
		t.Fatalf("expected 1 fetch while cached, got %d", got)
	}
	now = now.Add(2 * time.Hour) // past the 1h TTL
	cache.get(context.Background(), "vid")
	if got := atomic.LoadInt32(&calls); got != 2 {
		t.Fatalf("expected a re-fetch after expiry, got %d calls", got)
	}
}

func TestSegmentCacheEvictsOldest(t *testing.T) {
	cache := newSegmentCache(func(_ context.Context, _ string) []sponsorSegment { return nil })
	cache.max = 2
	cache.get(context.Background(), "a")
	cache.get(context.Background(), "b")
	cache.get(context.Background(), "c")
	if _, ok := cache.entries["a"]; ok {
		t.Error("expected oldest entry 'a' to be evicted")
	}
	if len(cache.entries) != 2 {
		t.Errorf("expected cache to hold 2 entries, got %d", len(cache.entries))
	}
}
