package app

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"sort"
	"sync"
	"time"
)

// sponsorSegment is a time range within a video that SponsorBlock recommends
// skipping, together with the reason it exists.
type sponsorSegment struct {
	Start    float64 `json:"start"`
	End      float64 `json:"end"`
	Category string  `json:"category"`
}

// sponsorCategories are the segment categories KoalaParty asks SponsorBlock about.
// Only "skip" action segments are used; the room decides which of these to act on.
var sponsorCategories = []string{"sponsor", "selfpromo", "interaction", "intro", "outro", "preview", "music_offtopic"}

var sponsorBlockClient = &http.Client{Timeout: 5 * time.Second}

// sponsorAPIBase is the SponsorBlock API root. It is a var so tests can point it at
// a local server. The segment data is licensed CC BY-NC-SA 4.0 and requires
// attribution wherever it is surfaced.
var sponsorAPIBase = "https://sponsor.ajay.app"

// fetchSponsorSegments returns the SponsorBlock skip segments for a video. It uses
// the privacy-preserving hash-prefix endpoint, so the full video ID is never sent
// to SponsorBlock — only the first four hex characters of its SHA-256, which match
// many videos. Any error, timeout, or absence yields nil so the caller degrades to
// no skipping.
func fetchSponsorSegments(ctx context.Context, videoID string) []sponsorSegment {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	sum := sha256.Sum256([]byte(videoID))
	prefix := hex.EncodeToString(sum[:])[:4]
	categories, _ := json.Marshal(sponsorCategories)
	endpoint := fmt.Sprintf("%s/api/skipSegments/%s?categories=%s&actionTypes=%s",
		sponsorAPIBase, prefix, url.QueryEscape(string(categories)), url.QueryEscape(`["skip"]`))
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil
	}
	req.Header.Set("Accept", "application/json")
	resp, err := sponsorBlockClient.Do(req)
	if err != nil {
		return nil
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil // 404 simply means no segments exist for this hash prefix.
	}
	var payload []struct {
		VideoID  string `json:"videoID"`
		Segments []struct {
			Category   string    `json:"category"`
			ActionType string    `json:"actionType"`
			Segment    []float64 `json:"segment"`
		} `json:"segments"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 512<<10)).Decode(&payload); err != nil {
		return nil
	}
	var out []sponsorSegment
	for _, video := range payload {
		if video.VideoID != videoID {
			continue // Other videos sharing our hash prefix.
		}
		for _, s := range video.Segments {
			if s.ActionType != "skip" || len(s.Segment) != 2 {
				continue
			}
			start, end := s.Segment[0], s.Segment[1]
			if end <= start || start < 0 || !contains(sponsorCategories, s.Category) {
				continue
			}
			out = append(out, sponsorSegment{Start: start, End: end, Category: s.Category})
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Start < out[j].Start })
	return out
}

// segmentCache memoizes segment lookups so repeated joins and rebroadcasts for the
// same video do not each hit SponsorBlock. Entries expire after ttl; an empty result
// is cached too (as a nil slice) so videos with no segments are not re-fetched.
type segmentCache struct {
	mu      sync.Mutex
	entries map[string]segmentCacheEntry
	order   []string // insertion order, for simple size-capped eviction
	ttl     time.Duration
	max     int
	now     func() time.Time
	fetch   func(ctx context.Context, videoID string) []sponsorSegment
}

type segmentCacheEntry struct {
	segments []sponsorSegment
	expires  time.Time
}

func newSegmentCache(fetch func(ctx context.Context, videoID string) []sponsorSegment) *segmentCache {
	return &segmentCache{
		entries: map[string]segmentCacheEntry{},
		ttl:     time.Hour,
		max:     2048,
		now:     time.Now,
		fetch:   fetch,
	}
}

// peek returns cached segments without ever fetching, so snapshot building stays
// fast and side-effect free. It returns nil when nothing fresh is cached.
func (c *segmentCache) peek(videoID string) []sponsorSegment {
	c.mu.Lock()
	defer c.mu.Unlock()
	if entry, ok := c.entries[videoID]; ok && c.now().Before(entry.expires) {
		return entry.segments
	}
	return nil
}

func (c *segmentCache) get(ctx context.Context, videoID string) []sponsorSegment {
	c.mu.Lock()
	if entry, ok := c.entries[videoID]; ok && c.now().Before(entry.expires) {
		c.mu.Unlock()
		return entry.segments
	}
	c.mu.Unlock()

	segments := c.fetch(ctx, videoID)

	c.mu.Lock()
	if _, ok := c.entries[videoID]; !ok {
		c.order = append(c.order, videoID)
		for len(c.order) > c.max {
			delete(c.entries, c.order[0])
			c.order = c.order[1:]
		}
	}
	c.entries[videoID] = segmentCacheEntry{segments: segments, expires: c.now().Add(c.ttl)}
	c.mu.Unlock()
	return segments
}

// enrichSegments fetches the SponsorBlock segments for a freshly activated video in
// the background (populating the cache) and, once resolved, rebroadcasts the room so
// every client receives them and can skip in sync. Runs in its own goroutine; all
// failures are silent so a missing or slow SponsorBlock never affects playback.
func (a *application) enrichSegments(room, videoID string) {
	// A panic in a bare goroutine would crash the whole process, unlike one inside an
	// HTTP handler; never let background work take the server down.
	defer func() {
		if r := recover(); r != nil {
			fmt.Fprintf(os.Stderr, `{"level":"error","message":"enrichSegments panic","room":%q,"error":"%v"}`+"\n", room, r)
		}
	}()
	if a.segments == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	a.segments.get(ctx, videoID) // populate the cache
	if !a.hub.activeRoom(room) {
		return
	}
	if s, err := a.snapshot(ctx, room, ""); err == nil {
		a.hub.broadcast(room, s)
	}
}
