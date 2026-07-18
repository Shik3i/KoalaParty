package app

import (
	"fmt"
	"net"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

type config struct {
	addr              string
	dbPath            string
	webRoot           string
	sessionTTL        time.Duration
	cookieSecure      bool
	trustedOrigins    map[string]bool
	trustedProxies    []*net.IPNet
	activityMaxAge    time.Duration
	activityMaxEvents int
	roomMaxIdle       time.Duration
	publicRooms       bool
	production        bool
}

func loadConfig() (config, error) {
	production, err := parseBool("KOALAPARTY_PRODUCTION", false)
	if err != nil {
		return config{}, err
	}
	cookieSecure, err := parseBool("KOALAPARTY_COOKIE_SECURE", false)
	if err != nil {
		return config{}, err
	}
	publicRooms, err := parseBool("KOALAPARTY_PUBLIC_ROOMS", false)
	if err != nil {
		return config{}, err
	}
	c := config{
		addr:           env("KOALAPARTY_ADDR", ":8080"),
		dbPath:         env("KOALAPARTY_DB", "koalaparty.db"),
		webRoot:        env("KOALAPARTY_WEB_ROOT", "../frontend/build"),
		cookieSecure:   cookieSecure,
		trustedOrigins: map[string]bool{},
		publicRooms:    publicRooms,
		production:     production,
	}
	if c.sessionTTL, err = parseDuration("KOALAPARTY_SESSION_TTL", "168h"); err != nil {
		return config{}, err
	}
	if c.activityMaxAge, err = parseDuration("KOALAPARTY_ACTIVITY_MAX_AGE", "720h"); err != nil {
		return config{}, err
	}
	if c.roomMaxIdle, err = parseDuration("KOALAPARTY_ROOM_MAX_IDLE", "8760h"); err != nil {
		return config{}, err
	}
	if c.activityMaxEvents, err = strconv.Atoi(env("KOALAPARTY_ACTIVITY_MAX_EVENTS", "200")); err != nil || c.activityMaxEvents < 10 {
		return config{}, fmt.Errorf("KOALAPARTY_ACTIVITY_MAX_EVENTS must be an integer of at least 10")
	}
	for _, raw := range strings.Split(env("KOALAPARTY_TRUSTED_ORIGINS", "http://localhost:5173,http://localhost:8080"), ",") {
		origin := strings.TrimSpace(raw)
		if origin == "" {
			return config{}, fmt.Errorf("KOALAPARTY_TRUSTED_ORIGINS contains an empty origin")
		}
		u, parseErr := url.Parse(origin)
		if parseErr != nil || u.Scheme == "" || u.Host == "" || u.Path != "" || u.RawQuery != "" || u.Fragment != "" {
			return config{}, fmt.Errorf("KOALAPARTY_TRUSTED_ORIGINS contains invalid origin %q", origin)
		}
		if production && (u.Scheme != "https" || u.Hostname() == "localhost" || net.ParseIP(u.Hostname()) != nil) {
			return config{}, fmt.Errorf("production trusted origin must use HTTPS and a hostname: %q", origin)
		}
		c.trustedOrigins[origin] = true
	}
	rawProxies := os.Getenv("KOALAPARTY_TRUSTED_PROXIES")
	if rawProxies == "" {
		rawProxies = "0.0.0.0/0,::/0"
	}
	for _, raw := range strings.Split(strings.TrimSpace(rawProxies), ",") {
		raw = strings.TrimSpace(raw)
		if raw == "" {
			continue
		}
		if !strings.Contains(raw, "/") {
			if ip := net.ParseIP(raw); ip != nil {
				if ip.To4() != nil {
					raw += "/32"
				} else {
					raw += "/128"
				}
			}
		}
		_, network, parseErr := net.ParseCIDR(raw)
		if parseErr != nil {
			return config{}, fmt.Errorf("KOALAPARTY_TRUSTED_PROXIES contains invalid IP or CIDR %q", raw)
		}
		c.trustedProxies = append(c.trustedProxies, network)
	}
	if production && !c.cookieSecure {
		return config{}, fmt.Errorf("KOALAPARTY_COOKIE_SECURE must be true in production")
	}
	return c, nil
}

func parseDuration(key, fallback string) (time.Duration, error) {
	value := env(key, fallback)
	duration, err := time.ParseDuration(value)
	if err != nil || duration <= 0 {
		return 0, fmt.Errorf("%s must be a positive duration, got %q", key, value)
	}
	return duration, nil
}

func parseBool(key string, fallback bool) (bool, error) {
	value := os.Getenv(key)
	if value == "" {
		return fallback, nil
	}
	parsed, err := strconv.ParseBool(value)
	if err != nil {
		return false, fmt.Errorf("%s must be true or false, got %q", key, value)
	}
	return parsed, nil
}
