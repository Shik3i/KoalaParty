package app

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func roomCommandAs(t *testing.T, a *application, room string, p principal, kind string, payload any) (snapshot, error) {
	t.Helper()
	raw, _ := json.Marshal(payload)
	s, _ := a.snapshot(t.Context(), room, p.IdentityID)
	return a.applyCommand(t.Context(), room, p, command{Type: kind, ExpectedRevision: s.Revision, ExpectedPlaybackRevision: &s.Playback.Revision, Payload: raw})
}

func TestRoomModesSetMemberDefaultsButOverridesWin(t *testing.T) {
	a := testApp(t)
	cookie, owner := exchange(t, a, "323e4567-e89b-42d3-a456-426614174001", strings.Repeat("a", 43))
	_, guest := exchange(t, a, "323e4567-e89b-42d3-a456-426614174002", strings.Repeat("b", 43))
	room := createTestRoom(t, a, cookie, owner)
	if _, err := a.joinAndSnapshot(t.Context(), room, guest); err != nil {
		t.Fatal(err)
	}
	if _, err := roomCommandAs(t, a, room, guest, "room.mode", map[string]string{"mode": "cinema"}); err != errDenied {
		t.Fatalf("members must not change the mode: %v", err)
	}
	s, err := roomCommandAs(t, a, room, owner, "room.mode", map[string]string{"mode": "cinema"})
	if err != nil || s.Mode != "cinema" {
		t.Fatalf("set cinema mode: %q %v", s.Mode, err)
	}
	if _, err = roomCommandAs(t, a, room, guest, "player.play", map[string]float64{"position": 1}); err != errDenied {
		t.Fatalf("cinema members must not control playback: %v", err)
	}
	if _, err = roomCommandAs(t, a, room, guest, "queue.add", map[string]string{"videoId": "abc12345678"}); err != nil {
		t.Fatalf("cinema members may still suggest videos: %v", err)
	}
	for _, m := range s.Members {
		if m.IdentityID == guest.IdentityID && (m.Permissions["playback.play_pause"] || !m.Permissions["queue.add"]) {
			t.Fatalf("snapshot permissions ignore the mode: %+v", m.Permissions)
		}
	}
	if _, err = roomCommandAs(t, a, room, owner, "member.permission", map[string]any{"identityId": guest.IdentityID, "permission": "playback.play_pause", "allowed": true}); err != nil {
		t.Fatal(err)
	}
	if _, err = roomCommandAs(t, a, room, guest, "player.play", map[string]float64{"position": 1}); err != nil {
		t.Fatalf("an explicit permission must override the mode: %v", err)
	}
	if _, err = roomCommandAs(t, a, room, owner, "room.mode", map[string]string{"mode": "host"}); err != nil {
		t.Fatal(err)
	}
	if _, err = roomCommandAs(t, a, room, guest, "queue.add", map[string]string{"videoId": "def12345678"}); err != errDenied {
		t.Fatalf("host-mode members must not add videos: %v", err)
	}
	if _, err = roomCommandAs(t, a, room, owner, "room.mode", map[string]string{"mode": "chaos"}); commandErrorCode(err) != "invalid_command" {
		t.Fatalf("unknown mode accepted: %v", err)
	}
}

func TestRoomSlugsAreValidatedUniqueAndResolvable(t *testing.T) {
	a := testApp(t)
	cookie, owner := exchange(t, a, "323e4567-e89b-42d3-a456-426614174003", strings.Repeat("c", 43))
	room := createTestRoom(t, a, cookie, owner)
	other := createTestRoom(t, a, cookie, owner)
	s, err := roomCommandAs(t, a, room, owner, "room.slug", map[string]string{"slug": " Film-Abend "})
	if err != nil || s.Slug != "film-abend" {
		t.Fatalf("slug: %q %v", s.Slug, err)
	}
	if _, err = roomCommandAs(t, a, other, owner, "room.slug", map[string]string{"slug": "FILM-abend"}); commandErrorCode(err) != "slug_taken" {
		t.Fatalf("duplicate slug accepted: %v", err)
	}
	for _, bad := range []string{"ab", "admin", "-lead", "trail-", "double--dash", "space here", "ümlaut"} {
		if _, err = roomCommandAs(t, a, other, owner, "room.slug", map[string]string{"slug": bad}); err == nil {
			t.Fatalf("slug %q accepted", bad)
		}
	}
	resolve := func(slug string) *httptest.ResponseRecorder {
		w := httptest.NewRecorder()
		r := httptest.NewRequest("GET", "/api/rooms/by-slug/"+slug, nil)
		r.SetPathValue("slug", slug)
		a.roomBySlug(w, r)
		return w
	}
	if w := resolve("Film-Abend"); w.Code != 200 || !strings.Contains(w.Body.String(), room) {
		t.Fatalf("resolve: %d %s", w.Code, w.Body.String())
	}
	if _, err = roomCommandAs(t, a, room, owner, "room.slug", map[string]string{"slug": ""}); err != nil {
		t.Fatal(err)
	}
	if w := resolve("film-abend"); w.Code != 404 {
		t.Fatalf("cleared slug still resolves: %d", w.Code)
	}
}

func TestCountdownHoldsNewVideosAndWaitPausesAreMarked(t *testing.T) {
	a := testApp(t)
	cookie, owner := exchange(t, a, "323e4567-e89b-42d3-a456-426614174004", strings.Repeat("d", 43))
	room := createTestRoom(t, a, cookie, owner)
	s, err := roomCommandAs(t, a, room, owner, "queue.play_now", map[string]any{"videoId": "abc12345678", "start": 10})
	if err != nil {
		t.Fatal(err)
	}
	if s.Playback.StartsAt <= s.ServerTime || s.Playback.StartsAt-s.ServerTime > 3500 || s.Playback.Position != 10 {
		t.Fatalf("new video did not count down: startsAt=%d server=%d position=%.2f", s.Playback.StartsAt, s.ServerTime, s.Playback.Position)
	}
	s, err = roomCommandAs(t, a, room, owner, "player.pause", map[string]any{"position": 10, "reason": "wait"})
	if err != nil || !s.Playback.AutoPaused || s.Playback.Status != "paused" {
		t.Fatalf("wait pause: %+v %v", s.Playback, err)
	}
	s, err = roomCommandAs(t, a, room, owner, "player.play", map[string]any{"position": 10, "countdown": 2})
	if err != nil || s.Playback.AutoPaused || s.Playback.StartsAt <= s.ServerTime {
		t.Fatalf("countdown resume: %+v %v", s.Playback, err)
	}
	if _, err = roomCommandAs(t, a, room, owner, "room.countdown", map[string]int{"seconds": 0}); err != nil {
		t.Fatal(err)
	}
	s, err = roomCommandAs(t, a, room, owner, "queue.play_now", map[string]any{"videoId": "def12345678"})
	if err != nil || s.Playback.StartsAt != 0 || s.CountdownSeconds != 0 {
		t.Fatalf("disabled countdown still delays: %+v %v", s.Playback, err)
	}
	if _, err = roomCommandAs(t, a, room, owner, "player.play", map[string]any{"position": 1, "countdown": 9}); commandErrorCode(err) != "invalid_command" {
		t.Fatalf("overlong countdown accepted: %v", err)
	}
	s, err = roomCommandAs(t, a, room, owner, "room.wait", map[string]bool{"enabled": false})
	if err != nil || s.WaitForAll {
		t.Fatalf("wait toggle: %v %v", s.WaitForAll, err)
	}
}

func TestPartySchedule(t *testing.T) {
	a := testApp(t)
	cookie, owner := exchange(t, a, "323e4567-e89b-42d3-a456-426614174005", strings.Repeat("e", 43))
	room := createTestRoom(t, a, cookie, owner)
	at := time.Now().Add(2 * time.Hour).Truncate(time.Second).UnixMilli()
	s, err := roomCommandAs(t, a, room, owner, "room.schedule", map[string]int64{"at": at})
	if err != nil || s.ScheduledAt != at {
		t.Fatalf("schedule: %d want %d err=%v", s.ScheduledAt, at, err)
	}
	if _, err = roomCommandAs(t, a, room, owner, "room.schedule", map[string]int64{"at": time.Now().AddDate(2, 0, 0).UnixMilli()}); commandErrorCode(err) != "invalid_schedule" {
		t.Fatalf("far-future schedule accepted: %v", err)
	}
	s, err = roomCommandAs(t, a, room, owner, "room.schedule", map[string]int64{"at": 0})
	if err != nil || s.ScheduledAt != 0 {
		t.Fatalf("clear schedule: %d %v", s.ScheduledAt, err)
	}
}

func TestBanListIsManagerOnly(t *testing.T) {
	a := testApp(t)
	cookie, owner := exchange(t, a, "323e4567-e89b-42d3-a456-426614174006", strings.Repeat("f", 43))
	guestCookie, guest := exchange(t, a, "323e4567-e89b-42d3-a456-426614174007", strings.Repeat("g", 43))
	_, rude := exchange(t, a, "323e4567-e89b-42d3-a456-426614174008", strings.Repeat("h", 43))
	room := createTestRoom(t, a, cookie, owner)
	for _, p := range []principal{guest, rude} {
		if _, err := a.joinAndSnapshot(t.Context(), room, p); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := roomCommandAs(t, a, room, owner, "member.ban", map[string]string{"identityId": rude.IdentityID}); err != nil {
		t.Fatal(err)
	}
	list := func(c *http.Cookie, p principal) *httptest.ResponseRecorder {
		w := httptest.NewRecorder()
		r := authed("GET", "/api/rooms/"+room+"/bans", nil, c, p.CSRF)
		r.SetPathValue("roomId", room)
		a.requireAuth(a.roomBans)(w, r)
		return w
	}
	if w := list(guestCookie, guest); w.Code != 403 {
		t.Fatalf("member saw bans: %d", w.Code)
	}
	if w := list(cookie, owner); w.Code != 200 || !strings.Contains(w.Body.String(), rude.IdentityID) {
		t.Fatalf("ban list: %d %s", w.Code, w.Body.String())
	}
}

func TestSavedQueuesRequireAnAccount(t *testing.T) {
	a := testApp(t)
	anonCookie, anon := exchange(t, a, "323e4567-e89b-42d3-a456-426614174009", strings.Repeat("i", 43))
	w := httptest.NewRecorder()
	a.requireAuth(a.savedQueues)(w, authed("GET", "/api/account/queues", nil, anonCookie, anon.CSRF))
	if w.Code != 403 {
		t.Fatalf("anonymous saved queues: %d", w.Code)
	}
	cookie, p, _ := accountPrincipal(t, a, "323e4567-e89b-42d3-a456-426614174010", "queue_user")
	w = httptest.NewRecorder()
	a.requireAuth(a.savedQueues)(w, authed("POST", "/api/account/queues", map[string]any{"name": " Filmabend ", "items": []map[string]any{{"videoId": "abc12345678", "start": 5}, {"videoId": "def12345678"}}}, cookie, p.CSRF))
	var created savedQueue
	_ = json.Unmarshal(w.Body.Bytes(), &created)
	if w.Code != 201 || created.Name != "Filmabend" || created.ItemCount != 2 {
		t.Fatalf("create: %d %s", w.Code, w.Body.String())
	}
	w = httptest.NewRecorder()
	a.requireAuth(a.savedQueues)(w, authed("POST", "/api/account/queues", map[string]any{"name": "Bad", "items": []map[string]any{{"videoId": "nope"}}}, cookie, p.CSRF))
	if w.Code != 400 {
		t.Fatalf("invalid video accepted: %d", w.Code)
	}
	w = httptest.NewRecorder()
	a.requireAuth(a.savedQueues)(w, authed("GET", "/api/account/queues", nil, cookie, p.CSRF))
	if w.Code != 200 || !strings.Contains(w.Body.String(), `"start":5`) {
		t.Fatalf("list: %d %s", w.Code, w.Body.String())
	}
	w = httptest.NewRecorder()
	r := authed("DELETE", "/api/account/queues/"+created.ID, nil, cookie, p.CSRF)
	r.SetPathValue("queueId", created.ID)
	a.requireAuth(a.deleteSavedQueue)(w, r)
	if w.Code != 204 {
		t.Fatalf("delete: %d", w.Code)
	}
}

func TestReactionHeatmapIsPerVideoAndEphemeral(t *testing.T) {
	h := newHub()
	c := &client{identity: "one", done: make(chan struct{})}
	if _, counted := h.recordReaction("ROOM", "YTabc", 12); counted {
		t.Fatal("heatmap counted without viewers")
	}
	if err := h.tryAdd("ROOM", c); err != nil {
		t.Fatal(err)
	}
	h.recordReaction("ROOM", "YTabc", 12)
	bucket, _ := h.recordReaction("ROOM", "YTabc", 14.9)
	if bucket != 2 {
		t.Fatalf("bucket=%d", bucket)
	}
	heat := h.heatmapFor("ROOM")
	if heat["mediaId"] != "YTabc" || heat["buckets"].(map[string]int)["2"] != 2 {
		t.Fatalf("heatmap: %+v", heat)
	}
	h.recordReaction("ROOM", "YTdef", 1)
	if h.heatmapFor("ROOM")["mediaId"] != "YTdef" {
		t.Fatal("heatmap did not reset for the next video")
	}
	h.remove("ROOM", c)
	if h.heatmapFor("ROOM") != nil {
		t.Fatal("heatmap outlived the room")
	}
}

func TestShortLinkPreviewUsesInvitationCopy(t *testing.T) {
	page := []byte(`<meta property="og:title" content="` + defaultPreviewTitle + `" />`)
	if !strings.Contains(string(renderIndex(page, "/r/filmabend", "")), roomPreviewTitle) {
		t.Fatal("short links should get the invitation preview")
	}
}
