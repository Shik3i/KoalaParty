package app

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"

	"golang.org/x/crypto/argon2"
)

type principal struct {
	IdentityID  string `json:"identityId"`
	AccountID   string `json:"accountId,omitempty"`
	DisplayName string `json:"displayName"`
	CSRF        string `json:"csrfToken"`
}
type identityRequest struct {
	ID          string `json:"id"`
	Secret      string `json:"secret"`
	DisplayName string `json:"displayName"`
}

var uuidPattern = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)

func randomToken(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}
func tokenHash(s string) string { h := sha256.Sum256([]byte(s)); return hex.EncodeToString(h[:]) }
func hashSecret(secret string) (string, error) {
	if len(secret) < 32 || len(secret) > 256 {
		return "", errors.New("secret must be 32 to 256 characters")
	}
	salt := make([]byte, 16)
	if _, e := rand.Read(salt); e != nil {
		return "", e
	}
	h := argon2.IDKey([]byte(secret), salt, 1, 64*1024, 4, 32)
	return fmt.Sprintf("$argon2id$v=19$m=65536,t=1,p=4$%s$%s", base64.RawStdEncoding.EncodeToString(salt), base64.RawStdEncoding.EncodeToString(h)), nil
}
func verifySecret(encoded, secret string) bool {
	parts := strings.Split(encoded, "$")
	if len(parts) != 6 || parts[1] != "argon2id" || parts[2] != "v=19" {
		return false
	}
	var m, t uint32
	var p uint8
	if _, e := fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d", &m, &t, &p); e != nil {
		return false
	}
	salt, e := base64.RawStdEncoding.DecodeString(parts[4])
	if e != nil {
		return false
	}
	expected, e := base64.RawStdEncoding.DecodeString(parts[5])
	if e != nil {
		return false
	}
	actual := argon2.IDKey([]byte(secret), salt, t, m, p, uint32(len(expected)))
	return subtle.ConstantTimeCompare(actual, expected) == 1
}

func (a *application) exchangeIdentity(w http.ResponseWriter, r *http.Request) {
	var in identityRequest
	if !decode(w, r, &in) {
		return
	}
	in.DisplayName = strings.TrimSpace(in.DisplayName)
	if !uuidPattern.MatchString(in.ID) || len(in.DisplayName) < 1 || len(in.DisplayName) > 32 {
		problem(w, 400, "invalid_identity", "Invalid identity fields.")
		return
	}
	var stored string
	err := a.db.QueryRow("SELECT secret_hash FROM identities WHERE id=?", in.ID).Scan(&stored)
	if errors.Is(err, sql.ErrNoRows) {
		stored, err = hashSecret(in.Secret)
		if err == nil {
			_, err = a.db.Exec("INSERT INTO identities(id,secret_hash,display_name,avatar_seed) VALUES(?,?,?,?)", in.ID, stored, in.DisplayName, in.ID[:8])
		}
	} else if err == nil && !verifySecret(stored, in.Secret) {
		problem(w, 401, "invalid_secret", "Identity secret was rejected.")
		return
	}
	if err != nil {
		problem(w, 500, "identity_failed", "Could not establish identity.")
		return
	}
	_, _ = a.db.Exec("UPDATE identities SET display_name=?,last_seen_at=CURRENT_TIMESTAMP WHERE id=?", in.DisplayName, in.ID)
	if current, authErr := a.authenticate(r); authErr == nil && current.IdentityID == in.ID {
		writeJSON(w, 200, current)
		return
	}
	a.issueSession(w, r, in.ID)
}

func (a *application) issueSession(w http.ResponseWriter, r *http.Request, identityID string) {
	p, err := a.principalByIdentity(identityID)
	if err != nil {
		problem(w, 500, "session_failed", "Could not create session.")
		return
	}
	token, err := randomToken(32)
	if err != nil {
		problem(w, 500, "session_failed", "Could not create session.")
		return
	}
	csrf, err := randomToken(24)
	if err != nil {
		problem(w, 500, "session_failed", "Could not create session.")
		return
	}
	expires := time.Now().Add(a.sessionTTL)
	_, err = a.db.Exec("INSERT INTO sessions(token_hash,identity_id,csrf_token,expires_at) VALUES(?,?,?,?)", tokenHash(token), identityID, csrf, expires.UTC().Format(time.RFC3339))
	if err != nil {
		problem(w, 500, "session_failed", "Could not create session.")
		return
	}
	http.SetCookie(w, &http.Cookie{Name: "kp_session", Value: token, Path: "/", HttpOnly: true, Secure: a.cookieSecure, SameSite: http.SameSiteLaxMode, Expires: expires})
	p.CSRF = csrf
	writeJSON(w, 200, p)
}
func (a *application) principalByIdentity(id string) (principal, error) {
	var p principal
	var account sql.NullString
	err := a.db.QueryRow("SELECT id,account_id,display_name FROM identities WHERE id=?", id).Scan(&p.IdentityID, &account, &p.DisplayName)
	if account.Valid {
		p.AccountID = account.String
	}
	return p, err
}
func (a *application) authenticate(r *http.Request) (principal, error) {
	c, e := r.Cookie("kp_session")
	if e != nil {
		return principal{}, e
	}
	var p principal
	var account sql.NullString
	e = a.db.QueryRow(`SELECT i.id,i.account_id,i.display_name,s.csrf_token FROM sessions s JOIN identities i ON i.id=s.identity_id WHERE s.token_hash=? AND s.expires_at>CURRENT_TIMESTAMP`, tokenHash(c.Value)).Scan(&p.IdentityID, &account, &p.DisplayName, &p.CSRF)
	if account.Valid {
		p.AccountID = account.String
	}
	return p, e
}
func (a *application) requireAuth(next func(http.ResponseWriter, *http.Request, principal)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		p, e := a.authenticate(r)
		if e != nil {
			problem(w, 401, "authentication_required", "Establish an identity first.")
			return
		}
		if r.Method != "GET" && r.Method != "HEAD" && r.Header.Get("X-CSRF-Token") != p.CSRF {
			problem(w, 403, "csrf_failed", "CSRF token is missing or invalid.")
			return
		}
		next(w, r, p)
	}
}
func (a *application) me(w http.ResponseWriter, r *http.Request, p principal) { writeJSON(w, 200, p) }
func decode(w http.ResponseWriter, r *http.Request, v any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, 64<<10)
	d := json.NewDecoder(r.Body)
	d.DisallowUnknownFields()
	if e := d.Decode(v); e != nil {
		problem(w, 400, "invalid_json", "Invalid request body.")
		return false
	}
	if e := d.Decode(&struct{}{}); !errors.Is(e, io.EOF) {
		problem(w, 400, "invalid_json", "Request body must contain one JSON object.")
		return false
	}
	return true
}
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
func problem(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, map[string]string{"code": code, "message": message})
}
