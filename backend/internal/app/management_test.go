package app

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func accountPrincipal(t *testing.T, a *application, identity, username string) (*http.Cookie, principal, string) {
	t.Helper()
	cookie, p := exchange(t, a, identity, strings.Repeat(identity[len(identity)-1:], 43))
	password := "secure-password-" + username
	w := httptest.NewRecorder()
	a.requireAuth(a.register)(w, authed("POST", "/api/accounts/register", credentials{Username: username, Password: password}, cookie, p.CSRF))
	if w.Code != 201 {
		t.Fatalf("register %s: %d %s", username, w.Code, w.Body.String())
	}
	if err := json.Unmarshal(w.Body.Bytes(), &p); err != nil {
		t.Fatal(err)
	}
	return cookie, p, password
}

func createTestRoom(t *testing.T, a *application, cookie *http.Cookie, p principal) string {
	t.Helper()
	w := httptest.NewRecorder()
	a.requireAuth(a.createRoom)(w, authed("POST", "/api/rooms", nil, cookie, p.CSRF))
	if w.Code != 201 {
		t.Fatalf("create room: %d %s", w.Code, w.Body.String())
	}
	var room map[string]string
	_ = json.Unmarshal(w.Body.Bytes(), &room)
	return room["id"]
}

func TestMyRoomsInvitationsOwnershipAndLifecycle(t *testing.T) {
	a := testApp(t)
	ownerCookie, owner, _ := accountPrincipal(t, a, "123e4567-e89b-42d3-a456-426614174011", "room_owner")
	memberCookie, memberP, _ := accountPrincipal(t, a, "123e4567-e89b-42d3-a456-426614174012", "room_member")
	room := createTestRoom(t, a, ownerCookie, owner)

	initial, err := a.snapshot(t.Context(), room, owner.IdentityID)
	if err != nil {
		t.Fatal(err)
	}
	private := command{Type: "room.visibility", ExpectedRevision: initial.Revision, Payload: json.RawMessage(`{"visibility":"private"}`)}
	if _, err = a.applyCommand(t.Context(), room, owner, private); err != nil {
		t.Fatal(err)
	}
	if _, err = a.joinAndSnapshot(t.Context(), room, memberP); err == nil || err.Error() != "not_allowed" {
		t.Fatalf("private room joined without invitation: %v", err)
	}

	invite := httptest.NewRecorder()
	r := authed("POST", "/api/rooms/"+room+"/invites", map[string]string{"username": "room_member"}, ownerCookie, owner.CSRF)
	r.SetPathValue("roomId", room)
	a.requireAuth(a.roomInvites)(invite, r)
	if invite.Code != 204 {
		t.Fatalf("invite: %d %s", invite.Code, invite.Body.String())
	}
	if _, err = a.joinAndSnapshot(t.Context(), room, memberP); err != nil {
		t.Fatal(err)
	}

	list := httptest.NewRecorder()
	r = authed("GET", "/api/rooms", nil, memberCookie, memberP.CSRF)
	a.requireAuth(a.myRooms)(list, r)
	if list.Code != 200 || !strings.Contains(list.Body.String(), room) || !strings.Contains(list.Body.String(), `"role":"member"`) {
		t.Fatalf("member room list: %d %s", list.Code, list.Body.String())
	}

	snapshot, err := a.snapshot(t.Context(), room, owner.IdentityID)
	if err != nil {
		t.Fatal(err)
	}
	transfer := command{Type: "room.transfer", ExpectedRevision: snapshot.Revision, Payload: json.RawMessage(`{"identityId":"` + memberP.IdentityID + `"}`)}
	if _, err = a.applyCommand(t.Context(), room, owner, transfer); err != nil {
		t.Fatal(err)
	}
	var ownerID, oldRole, newRole string
	_ = a.db.QueryRow("SELECT owner_identity_id FROM rooms WHERE id=?", room).Scan(&ownerID)
	_ = a.db.QueryRow("SELECT role FROM room_members WHERE room_id=? AND identity_id=?", room, owner.IdentityID).Scan(&oldRole)
	_ = a.db.QueryRow("SELECT role FROM room_members WHERE room_id=? AND identity_id=?", room, memberP.IdentityID).Scan(&newRole)
	if ownerID != memberP.IdentityID || oldRole != "admin" || newRole != "owner" {
		t.Fatalf("transfer state owner=%q old=%q new=%q", ownerID, oldRole, newRole)
	}

	leave := httptest.NewRecorder()
	r = authed("DELETE", "/api/rooms/"+room+"/membership", nil, ownerCookie, owner.CSRF)
	r.SetPathValue("roomId", room)
	a.requireAuth(a.leaveRoom)(leave, r)
	if leave.Code != 204 {
		t.Fatalf("leave: %d %s", leave.Code, leave.Body.String())
	}

	deleted := httptest.NewRecorder()
	r = authed("DELETE", "/api/rooms/"+room, nil, memberCookie, memberP.CSRF)
	r.SetPathValue("roomId", room)
	a.requireAuth(a.deleteRoom)(deleted, r)
	if deleted.Code != 204 {
		t.Fatalf("delete: %d %s", deleted.Code, deleted.Body.String())
	}
}

func TestAccountSelfServiceAndSessions(t *testing.T) {
	a := testApp(t)
	cookie, p, password := accountPrincipal(t, a, "123e4567-e89b-42d3-a456-426614174013", "account_user")
	room := createTestRoom(t, a, cookie, p)

	loginBody, _ := json.Marshal(credentials{Username: "account_user", Password: password})
	secondLogin := httptest.NewRecorder()
	a.login(secondLogin, httptest.NewRequest("POST", "/api/accounts/login", bytes.NewReader(loginBody)))
	if secondLogin.Code != 200 {
		t.Fatalf("second login: %d %s", secondLogin.Code, secondLogin.Body.String())
	}

	sessions := httptest.NewRecorder()
	a.requireAuth(a.accountSessions)(sessions, authed("GET", "/api/account/sessions", nil, cookie, p.CSRF))
	if sessions.Code != 200 {
		t.Fatalf("sessions: %d %s", sessions.Code, sessions.Body.String())
	}
	var listed []map[string]any
	_ = json.Unmarshal(sessions.Body.Bytes(), &listed)
	if len(listed) != 2 {
		t.Fatalf("sessions=%d want 2", len(listed))
	}

	revoke := httptest.NewRecorder()
	a.requireAuth(a.accountSessions)(revoke, authed("DELETE", "/api/account/sessions", nil, cookie, p.CSRF))
	if revoke.Code != 204 {
		t.Fatalf("revoke others: %d %s", revoke.Code, revoke.Body.String())
	}

	profile := httptest.NewRecorder()
	a.requireAuth(a.accountProfile)(profile, authed("PATCH", "/api/account/profile", map[string]string{"displayName": "New Koala"}, cookie, p.CSRF))
	if profile.Code != 200 || !strings.Contains(profile.Body.String(), "New Koala") {
		t.Fatalf("profile: %d %s", profile.Code, profile.Body.String())
	}
	_, restored := exchange(t, a, p.IdentityID, strings.Repeat("3", 43))
	if restored.DisplayName != "New Koala" {
		t.Fatalf("identity exchange replaced account profile with stale browser name: %q", restored.DisplayName)
	}

	newPassword := "a-new-secure-password"
	changed := httptest.NewRecorder()
	a.requireAuth(a.accountPassword)(changed, authed("POST", "/api/account/password", map[string]string{"currentPassword": password, "newPassword": newPassword}, cookie, p.CSRF))
	if changed.Code != 204 {
		t.Fatalf("password: %d %s", changed.Code, changed.Body.String())
	}
	sessions = httptest.NewRecorder()
	a.requireAuth(a.accountSessions)(sessions, authed("GET", "/api/account/sessions", nil, cookie, p.CSRF))
	listed = nil
	_ = json.Unmarshal(sessions.Body.Bytes(), &listed)
	if sessions.Code != 200 || len(listed) != 1 || listed[0]["current"] != true {
		t.Fatalf("password change did not revoke every other session: %d %s", sessions.Code, sessions.Body.String())
	}

	remove := httptest.NewRecorder()
	a.requireAuth(a.deleteOwnAccount)(remove, authed("DELETE", "/api/account", map[string]string{"password": newPassword}, cookie, p.CSRF))
	if remove.Code != 204 {
		t.Fatalf("delete account: %d %s", remove.Code, remove.Body.String())
	}
	var accounts, activeRooms int
	_ = a.db.QueryRow("SELECT count(*) FROM accounts WHERE id=?", p.AccountID).Scan(&accounts)
	_ = a.db.QueryRow("SELECT count(*) FROM rooms WHERE id=? AND deleted_at IS NULL", room).Scan(&activeRooms)
	if accounts != 0 || activeRooms != 0 {
		t.Fatalf("account cleanup accounts=%d activeRooms=%d", accounts, activeRooms)
	}
}
