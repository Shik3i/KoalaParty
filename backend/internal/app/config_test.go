package app

import (
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestProductionConfigurationRequiresHTTPSAndSecureCookies(t *testing.T) {
	t.Setenv("KOALAPARTY_PRODUCTION", "true")
	t.Setenv("KOALAPARTY_COOKIE_SECURE", "false")
	t.Setenv("KOALAPARTY_TRUSTED_ORIGINS", "https://party.example.com")
	if _, err := loadConfig(); err == nil {
		t.Fatal("insecure production cookie configuration was accepted")
	}
	t.Setenv("KOALAPARTY_COOKIE_SECURE", "true")
	t.Setenv("KOALAPARTY_TRUSTED_ORIGINS", "http://party.example.com")
	if _, err := loadConfig(); err == nil {
		t.Fatal("HTTP production origin was accepted")
	}
	t.Setenv("KOALAPARTY_TRUSTED_ORIGINS", "https://party.example.com")
	if _, err := loadConfig(); err != nil {
		t.Fatalf("valid production configuration rejected: %v", err)
	}
}

func TestConfigurationRejectsInvalidValues(t *testing.T) {
	t.Setenv("KOALAPARTY_SESSION_TTL", "soon")
	if _, err := loadConfig(); err == nil {
		t.Fatal("invalid duration was accepted")
	}
	t.Setenv("KOALAPARTY_SESSION_TTL", "1h")
	t.Setenv("KOALAPARTY_PUBLIC_ROOMS", "sometimes")
	if _, err := loadConfig(); err == nil {
		t.Fatal("invalid boolean was accepted")
	}
}

func TestRateLimiterUsesForwardedClientOnlyForTrustedProxy(t *testing.T) {
	t.Setenv("KOALAPARTY_TRUSTED_PROXIES", "127.0.0.1,10.0.0.0/8")
	cfg, err := loadConfig()
	if err != nil {
		t.Fatal(err)
	}
	limiter := newRateLimiter(1, time.Minute, cfg.trustedProxies)

	trusted := httptest.NewRequest("GET", "/", nil)
	trusted.RemoteAddr = "127.0.0.1:4321"
	trusted.Header.Set("X-Forwarded-For", "198.51.100.4, 10.1.2.3")
	if got := limiter.clientIP(trusted); got != "198.51.100.4" {
		t.Fatalf("trusted proxy client IP = %q", got)
	}

	untrusted := httptest.NewRequest("GET", "/", nil)
	untrusted.RemoteAddr = "203.0.113.8:4321"
	untrusted.Header.Set("X-Forwarded-For", "198.51.100.4")
	if got := limiter.clientIP(untrusted); got != "203.0.113.8" {
		t.Fatalf("spoofed forwarded address accepted: %q", got)
	}
}

func TestPublicRoomsDefaultToDisabled(t *testing.T) {
	cfg, err := loadConfig()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.publicRooms {
		t.Fatal("public rooms should be opt-in")
	}
}

func TestEnvironmentSettingsOverridePersistedAdminSettings(t *testing.T) {
	a := testApp(t)
	a.sessionTTL = 12 * time.Hour
	a.publicRooms = false
	a.settingOverrides = map[string]bool{
		"session_ttl":  true,
		"public_rooms": true,
	}
	if _, err := a.db.Exec(`
		INSERT INTO settings(key,value) VALUES
			('session_ttl','1h'),
			('public_rooms','true'),
			('activity_max_events','321')
		ON CONFLICT(key) DO UPDATE SET value=excluded.value`); err != nil {
		t.Fatal(err)
	}

	if err := a.loadSettingsFromDB(); err != nil {
		t.Fatal(err)
	}
	if a.sessionTTL != 12*time.Hour || a.publicRooms {
		t.Fatalf("environment-managed settings were overwritten: ttl=%s public=%v", a.sessionTTL, a.publicRooms)
	}
	if a.activityMaxEvents != 321 {
		t.Fatalf("database-managed setting was not loaded: %d", a.activityMaxEvents)
	}
}

func TestConfiguredEnvironmentOverridesAreExposedToApplication(t *testing.T) {
	t.Setenv("KOALAPARTY_SESSION_TTL", "12h")
	t.Setenv("KOALAPARTY_PUBLIC_ROOMS", "true")
	cfg, err := loadConfig()
	if err != nil {
		t.Fatal(err)
	}
	if !cfg.settingOverrides["session_ttl"] || !cfg.settingOverrides["public_rooms"] {
		t.Fatalf("missing environment override markers: %#v", cfg.settingOverrides)
	}
}

func TestAdminSettingsCannotChangeEnvironmentOverrides(t *testing.T) {
	a := testApp(t)
	a.sessionTTL = 12 * time.Hour
	a.activityMaxAge = 30 * 24 * time.Hour
	a.activityMaxEvents = 200
	a.roomMaxIdle = 365 * 24 * time.Hour
	a.publicRooms = false
	a.settingOverrides = map[string]bool{"session_ttl": true, "public_rooms": true}

	w := httptest.NewRecorder()
	r := httptest.NewRequest("POST", "/api/admin/settings", strings.NewReader(`{
		"sessionTTL":"1h",
		"activityMaxAge":"48h",
		"activityMaxEvents":50,
		"roomMaxIdle":"240h",
		"publicRooms":true
	}`))
	a.adminSettings(w, r, principal{IsAdmin: true})
	if w.Code != http.StatusNoContent {
		t.Fatalf("update settings: %d %s", w.Code, w.Body.String())
	}
	if a.sessionTTL != 12*time.Hour || a.publicRooms {
		t.Fatalf("admin changed environment-managed settings: ttl=%s public=%v", a.sessionTTL, a.publicRooms)
	}
	if a.activityMaxAge != 48*time.Hour || a.activityMaxEvents != 50 || a.roomMaxIdle != 240*time.Hour {
		t.Fatalf("admin-managed settings were not updated: age=%s events=%d idle=%s", a.activityMaxAge, a.activityMaxEvents, a.roomMaxIdle)
	}
}

func TestDiscoveryIsUnavailableUntilExplicitlyEnabled(t *testing.T) {
	a := testApp(t)
	w := httptest.NewRecorder()
	a.discover(w, httptest.NewRequest("GET", "/api/discover", nil))
	if w.Code != 404 {
		t.Fatalf("disabled discovery returned %d", w.Code)
	}
	a.setPublicRooms(true)
	w = httptest.NewRecorder()
	a.discover(w, httptest.NewRequest("GET", "/api/discover", nil))
	if w.Code != 200 || w.Body.String() != "[]\n" {
		t.Fatalf("enabled discovery returned %d %q", w.Code, w.Body.String())
	}
}

func TestRateLimiterTrustAllProxies(t *testing.T) {
	t.Setenv("KOALAPARTY_TRUSTED_PROXIES", "")
	cfg, err := loadConfig()
	if err != nil {
		t.Fatal(err)
	}
	limiter := newRateLimiter(1, time.Minute, cfg.trustedProxies)

	req := httptest.NewRequest("GET", "/", nil)
	req.RemoteAddr = "172.18.0.3:4321" // docker proxy IP
	req.Header.Set("X-Forwarded-For", "198.51.100.4, 10.1.2.3")
	if got := limiter.clientIP(req); got != "198.51.100.4" {
		t.Fatalf("trust all proxy client IP = %q, expected 198.51.100.4", got)
	}
}

func TestRateLimiterDefaultPrivateProxiesSecureAgainstSpoofing(t *testing.T) {
	t.Setenv("KOALAPARTY_TRUSTED_PROXIES", "")
	cfg, err := loadConfig()
	if err != nil {
		t.Fatal(err)
	}
	limiter := newRateLimiter(1, time.Minute, cfg.trustedProxies)

	req := httptest.NewRequest("GET", "/", nil)
	req.RemoteAddr = "172.18.0.3:4321"
	req.Header.Set("X-Forwarded-For", "1.1.1.1, 198.51.100.4")
	if got := limiter.clientIP(req); got != "198.51.100.4" {
		t.Fatalf("spoofed client IP accepted = %q, expected 198.51.100.4", got)
	}
}

func TestRateLimiterReportsItsActualWindow(t *testing.T) {
	limiter := newRateLimiter(1, time.Hour, nil)
	handler := limiter.wrap(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) })
	request := func() *httptest.ResponseRecorder {
		t.Helper()
		w := httptest.NewRecorder()
		r := httptest.NewRequest("POST", "/", nil)
		r.RemoteAddr = "203.0.113.10:4321"
		handler(w, r)
		return w
	}
	if w := request(); w.Code != http.StatusNoContent {
		t.Fatalf("first request: %d", w.Code)
	}
	w := request()
	retryAfter, err := strconv.Atoi(w.Header().Get("Retry-After"))
	if w.Code != http.StatusTooManyRequests || err != nil || retryAfter < 3599 || retryAfter > 3600 {
		t.Fatalf("rate limit response: status=%d retry-after=%q", w.Code, w.Header().Get("Retry-After"))
	}
}
