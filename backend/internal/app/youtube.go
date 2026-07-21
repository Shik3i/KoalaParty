package app

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
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

// enrichTitle resolves the real oEmbed title in the background and, if it
// differs from what is stored, updates the media row and rebroadcasts the room
// so every client sees the proper title. Runs in its own goroutine; all failures
// are silent and simply leave the placeholder in place.
func (a *application) enrichTitle(room, mediaID, videoID string) {
	// This runs in its own goroutine, so an unrecovered panic here would crash the
	// entire process (unlike a panic inside an HTTP handler, which net/http
	// recovers per-request). Never let background work take the server down.
	defer func() {
		if r := recover(); r != nil {
			fmt.Fprintf(os.Stderr, `{"level":"error","message":"enrichTitle panic","room":%q,"error":"%v"}`+"\n", room, r)
		}
	}()
	if a.fetchTitle == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 6*time.Second)
	defer cancel()
	title := a.fetchTitle(ctx, videoID)
	if title == "" {
		return
	}
	if len(title) > 200 {
		title = strings.TrimSpace(title[:200])
	}
	res, err := a.db.Exec("UPDATE media_items SET title=? WHERE id=? AND title<>?", title, mediaID, title)
	if err != nil {
		return
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return
	}
	rows, err := a.db.QueryContext(ctx, `SELECT DISTINCT room_id FROM (
		SELECT room_id FROM playback_states WHERE current_media_id=?
		UNION SELECT room_id FROM room_queue_items WHERE media_id=?
	)`, mediaID, mediaID)
	if err != nil {
		return
	}
	var affectedRooms []string
	for rows.Next() {
		var affectedRoom string
		if rows.Scan(&affectedRoom) != nil {
			rows.Close()
			return
		}
		affectedRooms = append(affectedRooms, affectedRoom)
	}
	if rows.Close() != nil || rows.Err() != nil {
		return
	}
	for _, affectedRoom := range affectedRooms {
		if !a.hub.activeRoom(affectedRoom) {
			continue
		}
		if s, snapshotErr := a.snapshot(ctx, affectedRoom, ""); snapshotErr == nil {
			a.hub.broadcast(affectedRoom, s)
		}
	}
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
