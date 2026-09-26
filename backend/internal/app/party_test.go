package app

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
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
		r := httptest.NewRequest("GET", "/api/room-links/"+slug, nil)
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

func TestPublicDiscoveryNeverShowsCustomRoomNames(t *testing.T) {
	a := testApp(t)
	a.setPublicRooms(true)
	cookie, owner, _ := accountPrincipal(t, a, "323e4567-e89b-42d3-a456-426614174020", "discover_owner")
	room := createTestRoom(t, a, cookie, owner)
	if _, err := roomCommandAs(t, a, room, owner, "room.visibility", map[string]string{"visibility": "public"}); err != nil {
		t.Fatal(err)
	}
	if _, err := roomCommandAs(t, a, room, owner, "room.rename", map[string]string{"name": "Unmoderated text"}); err != nil {
		t.Fatal(err)
	}
	w := httptest.NewRecorder()
	a.discover(w, httptest.NewRequest("GET", "/api/discover", nil))
	if w.Code != 200 || strings.Contains(w.Body.String(), "Unmoderated text") || !strings.Contains(w.Body.String(), roomLabel(room)) {
		t.Fatalf("discover exposed a custom name: %d %s", w.Code, w.Body.String())
	}
}

func TestSameSecondEventsKeepInsertionOrder(t *testing.T) {
	a := testApp(t)
	cookie, owner := exchange(t, a, "323e4567-e89b-42d3-a456-426614174031", strings.Repeat("q", 43))
	room := createTestRoom(t, a, cookie, owner)
	names := []string{"one", "two", "three", "four", "five", "six"}
	for _, name := range names {
		if _, err := roomCommandAs(t, a, room, owner, "room.rename", map[string]string{"name": name}); err != nil {
			t.Fatal(err)
		}
	}
	s, err := a.snapshot(t.Context(), room, owner.IdentityID)
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, e := range s.Events {
		if e.Type == "room.rename" {
			got = append(got, e.Payload["name"].(string))
		}
	}
	if strings.Join(got, ",") != strings.Join(names, ",") {
		t.Fatalf("events out of order: %v", got)
	}
}

func TestTitleLookupsSkipKnownTitlesAndBatchBroadcasts(t *testing.T) {
	a := testApp(t)
	var mu sync.Mutex
	lookups := map[string]int{}
	a.fetchTitle = func(_ context.Context, id string) string {
		mu.Lock()
		defer mu.Unlock()
		lookups[id]++
		return "Title " + id
	}
	cookie, owner := exchange(t, a, "323e4567-e89b-42d3-a456-426614174032", strings.Repeat("r", 43))
	room := createTestRoom(t, a, cookie, owner)
	ids := []string{}
	for i := range 8 {
		ids = append(ids, fmt.Sprintf("vid%08d", i))
	}
	items := []map[string]string{}
	for _, id := range ids {
		items = append(items, map[string]string{"videoId": id})
	}
	if _, err := roomCommandAs(t, a, room, owner, "queue.add", map[string]any{"items": items}); err != nil {
		t.Fatal(err)
	}
	a.enrichTitles(ids)
	a.enrichTitles(ids)
	s, _ := a.snapshot(t.Context(), room, owner.IdentityID)
	for _, item := range s.Queue {
		if item.Media.Title != "Title "+item.Media.ProviderID {
			t.Fatalf("title not enriched: %+v", item.Media)
		}
	}
	mu.Lock()
	defer mu.Unlock()
	for _, id := range ids {
		// The background lookup from queue.add may race the explicit calls, but a
		// resolved title is never requested again.
		if lookups[id] < 1 || lookups[id] > 2 {
			t.Fatalf("%s looked up %d times", id, lookups[id])
		}
	}
}

func TestPlayNextBeatsVotesUntilPlayed(t *testing.T) {
	a := testApp(t)
	cookie, owner := exchange(t, a, "323e4567-e89b-42d3-a456-426614174041", strings.Repeat("s", 43))
	room := createTestRoom(t, a, cookie, owner)
	for _, id := range []string{"aaaaaaaaaa1", "bbbbbbbbbb2", "ccccccccccc"} {
		if _, err := roomCommandAs(t, a, room, owner, "queue.add", map[string]string{"videoId": id}); err != nil {
			t.Fatal(err)
		}
	}
	s, _ := a.snapshot(t.Context(), room, owner.IdentityID)
	// aaaaaaaaaa1 started playing; bbbbbbbbbb2 and ccccccccccc are queued.
	var voted string
	for _, item := range s.Queue {
		if item.Media.ProviderID == "ccccccccccc" {
			voted = item.ID
		}
	}
	if _, err := roomCommandAs(t, a, room, owner, "queue.vote", map[string]string{"itemId": voted}); err != nil {
		t.Fatal(err)
	}
	s, err := roomCommandAs(t, a, room, owner, "queue.add", map[string]any{"videoId": "ddddddddddd", "position": 0})
	if err != nil {
		t.Fatal(err)
	}
	if s.Queue[0].Media.ProviderID != "ddddddddddd" || !s.Queue[0].Next || s.Queue[1].Media.ProviderID != "ccccccccccc" {
		t.Fatalf("play next must lead the queue ahead of votes: %+v", s.Queue)
	}
	// Moving an item to the top with a pin works the same way.
	ids := []string{}
	for _, item := range s.Queue {
		ids = append(ids, item.ID)
	}
	last := ids[len(ids)-1]
	order := append([]string{last}, ids[:len(ids)-1]...)
	if s, err = roomCommandAs(t, a, room, owner, "queue.reorder", map[string]any{"itemIds": order, "pin": last}); err != nil {
		t.Fatal(err)
	}
	if !s.Queue[0].Next && !s.Queue[1].Next {
		t.Fatalf("pinned items lost: %+v", s.Queue)
	}
	if _, err = roomCommandAs(t, a, room, owner, "queue.reorder", map[string]any{"itemIds": order, "pin": "unknown"}); err == nil {
		t.Fatal("pinning an unknown item must fail")
	}
	s, err = roomCommandAs(t, a, room, owner, "queue.skip", map[string]any{})
	if err != nil || s.Playback.Media == nil || s.Playback.Media.ProviderID == "ccccccccccc" {
		t.Fatalf("a pinned item must play before the voted one: %+v %v", s.Playback.Media, err)
	}
}

func TestReportedDurationStopsTheClockAtTheEnd(t *testing.T) {
	a := testApp(t)
	cookie, owner := exchange(t, a, "323e4567-e89b-42d3-a456-426614174042", strings.Repeat("t", 43))
	_, guest := exchange(t, a, "323e4567-e89b-42d3-a456-426614174043", strings.Repeat("u", 43))
	room := createTestRoom(t, a, cookie, owner)
	if _, err := roomCommandAs(t, a, room, owner, "room.countdown", map[string]int{"seconds": 0}); err != nil {
		t.Fatal(err)
	}
	// The test room starts with a cued video; play it.
	if _, err := roomCommandAs(t, a, room, owner, "player.play", map[string]float64{"position": 0}); err != nil {
		t.Fatal(err)
	}
	if _, err := a.joinAndSnapshot(t.Context(), room, guest); err != nil {
		t.Fatal(err)
	}
	report := func(p principal, media string, duration float64) {
		raw, _ := json.Marshal(map[string]any{"mediaId": media, "duration": duration})
		a.handleDuration(t.Context(), &client{send: make(chan any, 4), done: make(chan struct{})}, room, p, command{Type: "media.duration", Payload: raw})
	}
	if _, err := roomCommandAs(t, a, room, owner, "member.permission", map[string]any{"identityId": guest.IdentityID, "permission": "queue.skip", "allowed": false}); err != nil {
		t.Fatal(err)
	}
	report(guest, "YTjNQXAC9IVRw", 1)
	report(owner, "YTother00000", 5)
	s, _ := a.snapshot(t.Context(), room, owner.IdentityID)
	if s.Playback.Duration != 0 {
		t.Fatalf("unauthorised or foreign duration stored: %v", s.Playback.Duration)
	}
	report(owner, "YTjNQXAC9IVRw", 30)
	report(owner, "YTjNQXAC9IVRw", 2)
	if _, err := a.db.Exec("UPDATE playback_states SET updated_at=strftime('%Y-%m-%d %H:%M:%f','now','-1 hour') WHERE room_id=?", room); err != nil {
		t.Fatal(err)
	}
	s, _ = a.snapshot(t.Context(), room, owner.IdentityID)
	if s.Playback.Duration != 30 || s.Playback.Position != 30 {
		t.Fatalf("duration=%v position=%v, want both 30", s.Playback.Duration, s.Playback.Position)
	}
}

func TestBroadcastsCarryOnlyTheNewestEvents(t *testing.T) {
	a := testApp(t)
	cookie, owner := exchange(t, a, "323e4567-e89b-42d3-a456-426614174044", strings.Repeat("v", 43))
	room := createTestRoom(t, a, cookie, owner)
	for i := range broadcastEvents + 5 {
		if _, err := roomCommandAs(t, a, room, owner, "room.rename", map[string]string{"name": fmt.Sprintf("n%d", i)}); err != nil {
			t.Fatal(err)
		}
	}
	s, _ := a.snapshot(t.Context(), room, owner.IdentityID)
	c := &client{identity: owner.IdentityID, send: make(chan any, 4), done: make(chan struct{})}
	a.hub.mu.Lock()
	a.hub.rooms[room] = map[*client]struct{}{c: {}}
	a.hub.mu.Unlock()
	a.hub.broadcast(room, s)
	got := (<-c.send).(map[string]any)["payload"].(snapshot)
	if !got.EventsPartial || len(got.Events) != broadcastEvents || got.Events[len(got.Events)-1].ID != s.Events[len(s.Events)-1].ID {
		t.Fatalf("partial=%v events=%d", got.EventsPartial, len(got.Events))
	}
	if len(s.Events) <= broadcastEvents || s.EventsPartial {
		t.Fatal("the source snapshot must stay complete")
	}
}

func TestStaticNamesStayInsideTheWebRoot(t *testing.T) {
	for _, raw := range []string{"/../secret", "/..", "/a/../../b", "/_app/x.js"} {
		name, valid := staticName(raw)
		if !valid || strings.HasPrefix(name, "..") || strings.HasPrefix(name, "/") {
			t.Fatalf("%q mapped to %q (valid=%v)", raw, name, valid)
		}
	}
	if name, _ := staticName("/_app/x.js"); name != "_app/x.js" {
		t.Fatalf("regular file mapped to %q", name)
	}
}
