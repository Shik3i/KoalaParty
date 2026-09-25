package app

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode/utf8"
)

// The YouTube Data API is optional. With KOALAPARTY_YOUTUBE_API_KEY set, rooms
// get in-app search and playlist import; the server makes the requests, so
// viewers never contact the API and their queries are not tied to them there.
// Results are cached briefly to protect the operator's daily quota.

var youtubeAPIClient = &http.Client{Timeout: 6 * time.Second}
var youtubeAPIBase = "https://www.googleapis.com/youtube/v3"
var playlistIDPattern = regexp.MustCompile(`^[A-Za-z0-9_-]{10,64}$`)
var isoDuration = regexp.MustCompile(`^P(?:(\d+)D)?T?(?:(\d+)H)?(?:(\d+)M)?(?:(\d+)S)?$`)

type youtubeResult struct {
	VideoID   string `json:"videoId"`
	Title     string `json:"title"`
	Channel   string `json:"channel"`
	Thumbnail string `json:"thumbnail"`
	Duration  int    `json:"duration"`
}

type youtubeCacheEntry struct {
	results []youtubeResult
	title   string
	expires time.Time
}

type youtubeAPI struct {
	key   string
	mu    sync.Mutex
	cache map[string]youtubeCacheEntry
}

func newYouTubeAPI(key string) *youtubeAPI {
	if key == "" {
		return nil
	}
	return &youtubeAPI{key: key, cache: map[string]youtubeCacheEntry{}}
}

func (y *youtubeAPI) cached(key string) (youtubeCacheEntry, bool) {
	y.mu.Lock()
	defer y.mu.Unlock()
	entry, ok := y.cache[key]
	if !ok || time.Now().After(entry.expires) {
		delete(y.cache, key)
		return youtubeCacheEntry{}, false
	}
	return entry, true
}

func (y *youtubeAPI) store(key string, entry youtubeCacheEntry) {
	y.mu.Lock()
	defer y.mu.Unlock()
	if len(y.cache) >= 500 {
		for k, candidate := range y.cache {
			if time.Now().After(candidate.expires) || len(y.cache) >= 500 {
				delete(y.cache, k)
			}
		}
	}
	entry.expires = time.Now().Add(15 * time.Minute)
	y.cache[key] = entry
}

func (y *youtubeAPI) get(ctx context.Context, path string, params url.Values, out any) error {
	params.Set("key", y.key)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, youtubeAPIBase+path+"?"+params.Encode(), nil)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/json")
	resp, err := youtubeAPIClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return errors.New("youtube api status " + strconv.Itoa(resp.StatusCode))
	}
	return json.NewDecoder(io.LimitReader(resp.Body, 2<<20)).Decode(out)
}

func parseISODuration(raw string) int {
	match := isoDuration.FindStringSubmatch(raw)
	if match == nil {
		return 0
	}
	total := 0
	for i, unit := range []int{86400, 3600, 60, 1} {
		if n, err := strconv.Atoi(match[i+1]); err == nil {
			total += n * unit
		}
	}
	return total
}

type apiSnippet struct {
	Title        string `json:"title"`
	ChannelTitle string `json:"channelTitle"`
	Thumbnails   map[string]struct {
		URL string `json:"url"`
	} `json:"thumbnails"`
}

// details fills in durations and drops videos that cannot be embedded.
func (y *youtubeAPI) details(ctx context.Context, ids []string) (map[string]youtubeResult, error) {
	var payload struct {
		Items []struct {
			ID             string     `json:"id"`
			Snippet        apiSnippet `json:"snippet"`
			ContentDetails struct {
				Duration string `json:"duration"`
			} `json:"contentDetails"`
			Status struct {
				Embeddable bool `json:"embeddable"`
			} `json:"status"`
		} `json:"items"`
	}
	out := map[string]youtubeResult{}
	for start := 0; start < len(ids); start += 50 {
		end := min(start+50, len(ids))
		params := url.Values{"part": {"snippet,contentDetails,status"}, "id": {strings.Join(ids[start:end], ",")}, "maxResults": {"50"}}
		if err := y.get(ctx, "/videos", params, &payload); err != nil {
			return nil, err
		}
		for _, item := range payload.Items {
			if !item.Status.Embeddable || !youtubeID.MatchString(item.ID) {
				continue
			}
			out[item.ID] = youtubeResult{
				VideoID:   item.ID,
				Title:     item.Snippet.Title,
				Channel:   item.Snippet.ChannelTitle,
				Thumbnail: "https://i.ytimg.com/vi/" + item.ID + "/mqdefault.jpg",
				Duration:  parseISODuration(item.ContentDetails.Duration),
			}
		}
	}
	return out, nil
}

func (y *youtubeAPI) search(ctx context.Context, query string) ([]youtubeResult, error) {
	key := "search:" + strings.ToLower(query)
	if entry, ok := y.cached(key); ok {
		return entry.results, nil
	}
	var payload struct {
		Items []struct {
			ID struct {
				VideoID string `json:"videoId"`
			} `json:"id"`
		} `json:"items"`
	}
	params := url.Values{"part": {"snippet"}, "type": {"video"}, "videoEmbeddable": {"true"}, "maxResults": {"12"}, "q": {query}, "safeSearch": {"moderate"}}
	if err := y.get(ctx, "/search", params, &payload); err != nil {
		return nil, err
	}
	ids := []string{}
	for _, item := range payload.Items {
		ids = append(ids, item.ID.VideoID)
	}
	details, err := y.details(ctx, ids)
	if err != nil {
		return nil, err
	}
	results := []youtubeResult{}
	for _, id := range ids {
		if result, ok := details[id]; ok {
			results = append(results, result)
		}
	}
	y.store(key, youtubeCacheEntry{results: results})
	return results, nil
}

func (y *youtubeAPI) playlist(ctx context.Context, listID string) (string, []youtubeResult, error) {
	key := "playlist:" + listID
	if entry, ok := y.cached(key); ok {
		return entry.title, entry.results, nil
	}
	var meta struct {
		Items []struct {
			Snippet apiSnippet `json:"snippet"`
		} `json:"items"`
	}
	if err := y.get(ctx, "/playlists", url.Values{"part": {"snippet"}, "id": {listID}}, &meta); err != nil {
		return "", nil, err
	}
	if len(meta.Items) == 0 {
		return "", nil, errors.New("playlist not found")
	}
	ids := []string{}
	pageToken := ""
	for len(ids) < maxQueueItems {
		var page struct {
			NextPageToken string `json:"nextPageToken"`
			Items         []struct {
				ContentDetails struct {
					VideoID string `json:"videoId"`
				} `json:"contentDetails"`
			} `json:"items"`
		}
		params := url.Values{"part": {"contentDetails"}, "playlistId": {listID}, "maxResults": {"50"}}
		if pageToken != "" {
			params.Set("pageToken", pageToken)
		}
		if err := y.get(ctx, "/playlistItems", params, &page); err != nil {
			return "", nil, err
		}
		for _, item := range page.Items {
			if youtubeID.MatchString(item.ContentDetails.VideoID) && len(ids) < maxQueueItems {
				ids = append(ids, item.ContentDetails.VideoID)
			}
		}
		if page.NextPageToken == "" {
			break
		}
		pageToken = page.NextPageToken
	}
	details, err := y.details(ctx, ids)
	if err != nil {
		return "", nil, err
	}
	results := []youtubeResult{}
	for _, id := range ids {
		if result, ok := details[id]; ok {
			results = append(results, result)
		}
	}
	title := meta.Items[0].Snippet.Title
	y.store(key, youtubeCacheEntry{results: results, title: title})
	return title, results, nil
}

func (a *application) youtubeSearch(w http.ResponseWriter, r *http.Request, _ principal) {
	if a.youtube == nil {
		problem(w, 404, "search_disabled", "Search is not enabled on this server.")
		return
	}
	query := strings.TrimSpace(r.URL.Query().Get("q"))
	if query == "" || utf8.RuneCountInString(query) > 120 {
		problem(w, 400, "invalid_query", "Enter a search term of up to 120 characters.")
		return
	}
	results, err := a.youtube.search(r.Context(), query)
	if err != nil {
		problem(w, 502, "search_failed", "YouTube search is unavailable right now. Paste a link instead.")
		return
	}
	writeJSON(w, 200, results)
}

func (a *application) youtubePlaylist(w http.ResponseWriter, r *http.Request, _ principal) {
	if a.youtube == nil {
		problem(w, 404, "search_disabled", "Playlist import is not enabled on this server.")
		return
	}
	listID := strings.TrimSpace(r.URL.Query().Get("list"))
	if !playlistIDPattern.MatchString(listID) {
		problem(w, 400, "invalid_playlist", "That is not a valid YouTube playlist.")
		return
	}
	title, results, err := a.youtube.playlist(r.Context(), listID)
	if err != nil {
		problem(w, 502, "playlist_failed", "This playlist could not be loaded. It may be private.")
		return
	}
	writeJSON(w, 200, map[string]any{"title": title, "items": results})
}
