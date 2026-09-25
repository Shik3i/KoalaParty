package app

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func addCommand(t *testing.T, a *application, room string, p principal, revision int64, payload string) (snapshot, error) {
	t.Helper()
	return a.applyCommand(t.Context(), room, p, command{Type: "queue.add", ExpectedRevision: revision, Payload: json.RawMessage(payload)})
}

func TestFreshRoomStartsEmptyAndFirstAddPlaysImmediately(t *testing.T) {
	a := testApp(t)
	cookie, owner := exchange(t, a, "223e4567-e89b-42d3-a456-426614174001", strings.Repeat("a", 43))
	w := httptest.NewRecorder()
	a.requireAuth(a.createRoom)(w, authed("POST", "/api/rooms", nil, cookie, owner.CSRF))
	var created map[string]string
	_ = json.Unmarshal(w.Body.Bytes(), &created)
	room := created["id"]
	s, err := a.snapshot(t.Context(), room, owner.IdentityID)
	if err != nil || s.Playback.Media != nil {
		t.Fatalf("fresh room should be empty: media=%+v err=%v", s.Playback.Media, err)
	}
	s, err = addCommand(t, a, room, owner, s.Revision, `{"videoId":"abc12345678","start":42}`)
	if err != nil {
		t.Fatal(err)
	}
	if s.Playback.Media == nil || s.Playback.Media.ProviderID != "abc12345678" || s.Playback.Status != "playing" || s.Playback.Position < 42 || len(s.Queue) != 0 {
		t.Fatalf("first add did not start playing at its start time: %+v queue=%d", s.Playback, len(s.Queue))
	}
}

func TestQueueAddIgnoresUnrelatedRevisionChanges(t *testing.T) {
	a := testApp(t)
	cookie, owner := exchange(t, a, "223e4567-e89b-42d3-a456-426614174002", strings.Repeat("b", 43))
	room := createTestRoom(t, a, cookie, owner)
	s, _ := a.snapshot(t.Context(), room, owner.IdentityID)
	stale := s.Revision
	if _, err := a.applyCommand(t.Context(), room, owner, command{Type: "player.play", ExpectedRevision: s.Revision, Payload: json.RawMessage(`{"position":3}`)}); err != nil {
		t.Fatal(err)
	}
	s, err := addCommand(t, a, room, owner, stale, `{"videoId":"abc12345678"}`)
	if err != nil || len(s.Queue) != 1 {
		t.Fatalf("add lost to an unrelated play: queue=%d err=%v", len(s.Queue), err)
	}
	if s.Queue[0].AddedBy != "Calm Koala" {
		t.Fatalf("queue item does not say who added it: %q", s.Queue[0].AddedBy)
	}
	_, err = addCommand(t, a, room, owner, stale, `{"videoId":"abc12345678"}`)
	status, code, message := commandProblem(err)
	if status != http.StatusBadRequest || code != "already_queued" || !strings.Contains(message, "already in the queue") {
		t.Fatalf("duplicate add problem = %d %q %q", status, code, message)
	}
	if _, err = a.applyCommand(t.Context(), room, owner, command{Type: "queue.reorder", ExpectedRevision: stale, Payload: json.RawMessage(`{"itemIds":[]}`)}); commandErrorCode(err) != "stale_revision" {
		t.Fatalf("order-dependent commands must still reject stale revisions: %v", err)
	}
}

func TestBatchAddSkipsDuplicatesAndPlayNextInsertsFirst(t *testing.T) {
	a := testApp(t)
	cookie, owner := exchange(t, a, "223e4567-e89b-42d3-a456-426614174003", strings.Repeat("c", 43))
	room := createTestRoom(t, a, cookie, owner)
	s, _ := a.snapshot(t.Context(), room, owner.IdentityID)
	s, err := addCommand(t, a, room, owner, s.Revision, `{"items":[{"videoId":"aaa12345678"},{"videoId":"bbb12345678"},{"videoId":"aaa12345678"},{"videoId":"jNQXAC9IVRw"}]}`)
	if err != nil || len(s.Queue) != 2 {
		t.Fatalf("batch add: queue=%d err=%v", len(s.Queue), err)
	}
	s, err = addCommand(t, a, room, owner, s.Revision, `{"videoId":"ccc12345678","start":10,"position":0}`)
	if err != nil || s.Queue[0].Media.ProviderID != "ccc12345678" || s.Queue[0].Start != 10 {
		t.Fatalf("play next did not insert first: %+v err=%v", s.Queue, err)
	}
	s, err = a.applyCommand(t.Context(), room, owner, command{Type: "queue.skip", ExpectedRevision: s.Revision})
	if err != nil || s.Playback.Media.ProviderID != "ccc12345678" || s.Playback.Position < 10 {
		t.Fatalf("skip ignored the queued start time: %+v err=%v", s.Playback, err)
	}
	if _, err = addCommand(t, a, room, owner, s.Revision, `{"videoId":"ddd12345678","start":-1}`); commandErrorCode(err) != "invalid_command" {
		t.Fatalf("negative start accepted: %v", err)
	}
}

func TestVoteSkipNeedsMajorityOfConnectedViewers(t *testing.T) {
	a := testApp(t)
	cookie, owner := exchange(t, a, "223e4567-e89b-42d3-a456-426614174004", strings.Repeat("d", 43))
	_, guest := exchange(t, a, "223e4567-e89b-42d3-a456-426614174005", strings.Repeat("e", 43))
	_, third := exchange(t, a, "223e4567-e89b-42d3-a456-426614174006", strings.Repeat("f", 43))
	room := createTestRoom(t, a, cookie, owner)
	for _, p := range []principal{guest, third} {
		if _, err := a.joinAndSnapshot(t.Context(), room, p); err != nil {
			t.Fatal(err)
		}
	}
	for _, p := range []principal{owner, guest, third} {
		if err := a.hub.tryAdd(room, &client{identity: p.IdentityID, sessionHash: p.IdentityID, remoteIP: p.IdentityID, done: make(chan struct{})}); err != nil {
			t.Fatal(err)
		}
	}
	s, _ := a.snapshot(t.Context(), room, owner.IdentityID)
	s, _ = addCommand(t, a, room, owner, s.Revision, `{"videoId":"next1234567"}`)
	vote := command{Type: "queue.vote_skip", ExpectedRevision: 0}
	s, err := a.applyCommand(t.Context(), room, guest, vote)
	if err != nil || s.Playback.SkipVotes != 1 || s.Playback.SkipNeeded != 2 || s.Playback.Media.ProviderID != "jNQXAC9IVRw" {
		t.Fatalf("first vote: %+v err=%v", s.Playback, err)
	}
	if !s.forIdentity(guest.IdentityID).Playback.SkipVoted || s.forIdentity(third.IdentityID).Playback.SkipVoted {
		t.Fatal("skip vote personalization is wrong")
	}
	s, err = a.applyCommand(t.Context(), room, third, vote)
	if err != nil || s.Playback.Media.ProviderID != "next1234567" || s.Playback.SkipVotes != 0 {
		t.Fatalf("majority did not skip: %+v err=%v", s.Playback, err)
	}
}

func TestRoomRenameChangesLabelEverywhere(t *testing.T) {
	a := testApp(t)
	cookie, owner := exchange(t, a, "223e4567-e89b-42d3-a456-426614174007", strings.Repeat("g", 43))
	_, guest := exchange(t, a, "223e4567-e89b-42d3-a456-426614174008", strings.Repeat("h", 43))
	room := createTestRoom(t, a, cookie, owner)
	if _, err := a.joinAndSnapshot(t.Context(), room, guest); err != nil {
		t.Fatal(err)
	}
	rename := func(p principal, name string) (snapshot, error) {
		payload, _ := json.Marshal(map[string]string{"name": name})
		return a.applyCommand(t.Context(), room, p, command{Type: "room.rename", Payload: payload})
	}
	if _, err := rename(guest, "Hijacked"); err != errDenied {
		t.Fatalf("members must not rename rooms: %v", err)
	}
	s, err := rename(owner, "  Movie night\x07  ")
	if err != nil || s.Label != "Movie night" {
		t.Fatalf("rename: label=%q err=%v", s.Label, err)
	}
	if _, err = rename(owner, strings.Repeat("x", 61)); commandErrorCode(err) != "invalid_room_name" {
		t.Fatalf("overlong name accepted: %v", err)
	}
	s, err = rename(owner, "")
	if err != nil || s.Label != roomLabel(room) {
		t.Fatalf("clearing the name should restore the generated label: %q %v", s.Label, err)
	}
}

func TestAnonymousViewersCanRenameThemselves(t *testing.T) {
	a := testApp(t)
	cookie, p := exchange(t, a, "223e4567-e89b-42d3-a456-426614174009", strings.Repeat("i", 43))
	w := httptest.NewRecorder()
	a.requireAuth(a.accountProfile)(w, authed("PATCH", "/api/account/profile", map[string]string{"displayName": " Lisa "}, cookie, p.CSRF))
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), `"displayName":"Lisa"`) {
		t.Fatalf("anonymous rename: %d %s", w.Code, w.Body.String())
	}
}

func TestChatIsEphemeralAndBounded(t *testing.T) {
	h := newHub()
	c := &client{identity: "one", done: make(chan struct{})}
	h.appendChat("ROOM", chatMessage{ID: "early"})
	if len(h.chatHistory("ROOM")) != 0 {
		t.Fatal("chat stored for a room without viewers")
	}
	if err := h.tryAdd("ROOM", c); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < chatHistoryLimit+5; i++ {
		h.appendChat("ROOM", chatMessage{ID: string(rune('a' + i%26))})
	}
	if got := len(h.chatHistory("ROOM")); got != chatHistoryLimit {
		t.Fatalf("chat history=%d want %d", got, chatHistoryLimit)
	}
	if !h.setPresence("ROOM", "one", "buffering") || h.setPresence("ROOM", "one", "buffering") {
		t.Fatal("presence should only report changes")
	}
	h.remove("ROOM", c)
	if len(h.chatHistory("ROOM")) != 0 || len(h.presenceStates("ROOM")) != 0 {
		t.Fatal("chat or presence outlived the last viewer")
	}
	if got := cleanChatText(" hi\x00 there\nfriend\t "); got != "hi there\nfriend" {
		t.Fatalf("cleanChatText=%q", got)
	}
	if !h.allowChat("one", time.Now()) {
		t.Fatal("first chat message rate-limited")
	}
}

func TestIndexPreviewMetadata(t *testing.T) {
	page := []byte(`<meta property="og:image" content="/og-image.jpg" /><meta property="og:title" content="` + defaultPreviewTitle + `" /><meta property="og:description" content="` + defaultPreviewDescription + `" />`)
	home := string(renderIndex(page, "/", "https://party.example"))
	if !strings.Contains(home, `content="https://party.example/og-image.jpg"`) || !strings.Contains(home, defaultPreviewTitle) {
		t.Fatalf("home preview: %s", home)
	}
	room := string(renderIndex(page, "/room/ABCDEFGHIJKLMNOP", "https://party.example"))
	if !strings.Contains(room, roomPreviewTitle) || !strings.Contains(room, roomPreviewDescription) || strings.Contains(room, defaultPreviewTitle) {
		t.Fatalf("room preview: %s", room)
	}
}

func TestYouTubeSearchUsesServerSideAPI(t *testing.T) {
	var calls []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls = append(calls, r.URL.Path)
		if r.URL.Query().Get("key") != "secret" {
			w.WriteHeader(http.StatusForbidden)
			return
		}
		switch r.URL.Path {
		case "/search":
			_, _ = w.Write([]byte(`{"items":[{"id":{"videoId":"abc12345678"}},{"id":{"videoId":"blocked1234"}}]}`))
		case "/videos":
			_, _ = w.Write([]byte(`{"items":[{"id":"abc12345678","snippet":{"title":"Koalas","channelTitle":"Zoo"},"contentDetails":{"duration":"PT1H2M3S"},"status":{"embeddable":true}},{"id":"blocked1234","snippet":{"title":"No embed"},"contentDetails":{"duration":"PT1M"},"status":{"embeddable":false}}]}`))
		}
	}))
	defer server.Close()
	previous := youtubeAPIBase
	youtubeAPIBase = server.URL
	defer func() { youtubeAPIBase = previous }()

	a := testApp(t)
	cookie, p := exchange(t, a, "223e4567-e89b-42d3-a456-426614174010", strings.Repeat("j", 43))
	w := httptest.NewRecorder()
	a.requireAuth(a.youtubeSearch)(w, authed("GET", "/api/youtube/search?q=koala", nil, cookie, p.CSRF))
	if w.Code != http.StatusNotFound {
		t.Fatalf("search without an API key must be disabled: %d", w.Code)
	}
	a.youtube = newYouTubeAPI("secret")
	for range 2 {
		w = httptest.NewRecorder()
		a.requireAuth(a.youtubeSearch)(w, authed("GET", "/api/youtube/search?q=koala", nil, cookie, p.CSRF))
	}
	var results []youtubeResult
	_ = json.Unmarshal(w.Body.Bytes(), &results)
	if w.Code != 200 || len(results) != 1 || results[0].Duration != 3723 || results[0].Channel != "Zoo" {
		t.Fatalf("search results: %d %s", w.Code, w.Body.String())
	}
	if len(calls) != 2 {
		t.Fatalf("repeated search was not cached: calls=%v", calls)
	}
}
