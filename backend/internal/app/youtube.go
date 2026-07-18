package app

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// youtubeTitleClient performs the best-effort oEmbed metadata lookup. Titles are
// a nicety, so the timeout is short and every failure degrades to the caller's
// fallback.
var youtubeTitleClient = &http.Client{Timeout: 4 * time.Second}

// resolveMediaTitle returns the best available title for a video: the oEmbed
// title when metadata lookups are enabled and succeed, otherwise the caller's
// fallback, otherwise a stable placeholder. The result is always trimmed and
// capped to the storage limit.
func (a *application) resolveMediaTitle(ctx context.Context, videoID, fallback string) string {
	title := strings.TrimSpace(fallback)
	if a.fetchTitle != nil {
		if fetched := a.fetchTitle(ctx, videoID); fetched != "" {
			title = fetched
		}
	}
	if title == "" {
		title = "YouTube video " + videoID
	}
	if len(title) > 200 {
		title = strings.TrimSpace(title[:200])
	}
	return title
}

// fetchYouTubeTitle resolves the human-readable title for a video via YouTube's
// public oEmbed endpoint. It returns "" on any error so callers keep their
// placeholder. This is the only outbound request KoalaParty makes to YouTube and
// it can be disabled with KOALAPARTY_YOUTUBE_METADATA=false.
func fetchYouTubeTitle(ctx context.Context, videoID string) string {
	ctx, cancel := context.WithTimeout(ctx, 4*time.Second)
	defer cancel()
	endpoint := "https://www.youtube.com/oembed?format=json&url=" +
		url.QueryEscape("https://www.youtube.com/watch?v="+videoID)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return ""
	}
	req.Header.Set("Accept", "application/json")
	resp, err := youtubeTitleClient.Do(req)
	if err != nil {
		return ""
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return ""
	}
	var payload struct {
		Title string `json:"title"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 64<<10)).Decode(&payload); err != nil {
		return ""
	}
	return strings.TrimSpace(payload.Title)
}
