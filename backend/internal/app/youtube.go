package app

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

// youtubeTitleClient performs the best-effort oEmbed metadata lookup. Titles are
// a nicety, so the timeout is short and every failure degrades to the caller's
// fallback.
var youtubeTitleClient = &http.Client{Timeout: 4 * time.Second}

// fallbackTitle is the title stored immediately when a video is queued, before
// any network lookup: the caller's title, or a stable placeholder. It never
// blocks, so adding a video is always instant even when the server cannot reach
// YouTube. The real title is filled in afterwards by enrichTitle.
func fallbackTitle(clientTitle, videoID string) string {
	title := strings.TrimSpace(clientTitle)
	if title == "" {
		title = "YouTube video " + videoID
	}
	if len(title) > 200 {
		title = strings.TrimSpace(title[:200])
	}
	return title
}

// titleLookupBatch bounds parallel oEmbed requests and how many titles are
// collected before the affected rooms are rebroadcast once.
const titleLookupBatch = 6

// enrichTitles resolves real oEmbed titles in the background for videos that
// still carry the placeholder, updates the media rows and rebroadcasts each
// affected room once per batch, so importing a playlist does not flood viewers
// with a snapshot per video. All failures are silent and keep the placeholder.
func (a *application) enrichTitles(videoIDs []string) {
	// This runs in its own goroutine, so an unrecovered panic here would crash the
	// entire process (unlike a panic inside an HTTP handler, which net/http
	// recovers per-request). Never let background work take the server down.
	defer func() {
		if recover() != nil {
			loggerWithWriter(a.logger).Error("enrichTitles panic")
		}
	}()
	if a.fetchTitle == nil {
		return
	}
	pending := make([]string, 0, len(videoIDs))
	for _, videoID := range videoIDs {
		var title string
		// Titles already resolved for an earlier room need no second request.
		if a.db.QueryRow("SELECT coalesce(title,'') FROM media_items WHERE id=?", "YT"+videoID).Scan(&title) == nil && title != fallbackTitle("", videoID) {
			continue
		}
		pending = append(pending, videoID)
	}
	for start := 0; start < len(pending); start += titleLookupBatch {
		batch := pending[start:min(start+titleLookupBatch, len(pending))]
		titles := make([]string, len(batch))
		var wg sync.WaitGroup
		for i, videoID := range batch {
			wg.Add(1)
			go func() {
				defer wg.Done()
				defer func() {
					if recover() != nil {
						loggerWithWriter(a.logger).Error("title lookup panic")
					}
				}()
				ctx, cancel := context.WithTimeout(context.Background(), 6*time.Second)
				defer cancel()
				titles[i] = a.fetchTitle(ctx, videoID)
			}()
		}
		wg.Wait()
		affected := map[string]bool{}
		for i, videoID := range batch {
			title := titles[i]
			if title == "" {
				continue
			}
			if len(title) > 200 {
				title = strings.TrimSpace(title[:200])
			}
			mediaID := "YT" + videoID
			res, err := a.db.Exec("UPDATE media_items SET title=? WHERE id=? AND title<>?", title, mediaID, title)
			if err != nil {
				continue
			}
			if n, _ := res.RowsAffected(); n == 0 {
				continue
			}
			for _, room := range a.roomsShowingMedia(mediaID) {
				affected[room] = true
			}
		}
		for room := range affected {
			if !a.hub.activeRoom(room) {
				continue
			}
			if s, err := a.snapshot(context.Background(), room, ""); err == nil {
				a.hub.broadcast(room, s)
			}
		}
	}
}

// roomsShowingMedia lists rooms whose current video or queue contains mediaID.
func (a *application) roomsShowingMedia(mediaID string) []string {
	rows, err := a.db.Query(`SELECT DISTINCT room_id FROM (
		SELECT room_id FROM playback_states WHERE current_media_id=?
		UNION SELECT room_id FROM room_queue_items WHERE media_id=?
	)`, mediaID, mediaID)
	if err != nil {
		return nil
	}
	defer rows.Close()
	var rooms []string
	for rows.Next() {
		var room string
		if rows.Scan(&room) == nil {
			rooms = append(rooms, room)
		}
	}
	return rooms
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
