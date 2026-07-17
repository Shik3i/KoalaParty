package app

import (
	"net/http/httptest"
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

func TestDiscoveryIsUnavailableUntilExplicitlyEnabled(t *testing.T) {
	a := testApp(t)
	w := httptest.NewRecorder()
	a.discover(w, httptest.NewRequest("GET", "/api/discover", nil))
	if w.Code != 404 {
		t.Fatalf("disabled discovery returned %d", w.Code)
	}
	a.publicRooms = true
	w = httptest.NewRecorder()
	a.discover(w, httptest.NewRequest("GET", "/api/discover", nil))
	if w.Code != 200 || w.Body.String() != "[]\n" {
		t.Fatalf("enabled discovery returned %d %q", w.Code, w.Body.String())
	}
}
