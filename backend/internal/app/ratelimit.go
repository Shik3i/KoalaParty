package app

import (
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
)

type rateEntry struct {
	count int
	reset time.Time
}
type rateLimiter struct {
	mu             sync.Mutex
	limit          int
	window         time.Duration
	entries        map[string]rateEntry
	trustedProxies []*net.IPNet
}

func newRateLimiter(limit int, window time.Duration, trustedProxies []*net.IPNet) *rateLimiter {
	return &rateLimiter{limit: limit, window: window, entries: map[string]rateEntry{}, trustedProxies: trustedProxies}
}

func remoteIP(remoteAddr string) net.IP {
	host, _, err := net.SplitHostPort(remoteAddr)
	if err != nil {
		host = remoteAddr
	}
	return net.ParseIP(strings.TrimSpace(host))
}

func (l *rateLimiter) clientIP(r *http.Request) string {
	peer := remoteIP(r.RemoteAddr)
	trusted := false
	for _, network := range l.trustedProxies {
		if peer != nil && network.Contains(peer) {
			trusted = true
			break
		}
	}
	if trusted {
		parts := strings.Split(r.Header.Get("X-Forwarded-For"), ",")
		for i := len(parts) - 1; i >= 0; i-- {
			candidate := net.ParseIP(strings.TrimSpace(parts[i]))
			if candidate == nil {
				continue
			}
			candidateTrusted := false
			for _, network := range l.trustedProxies {
				if network.Contains(candidate) {
					candidateTrusted = true
					break
				}
			}
			if !candidateTrusted {
				return candidate.String()
			}
		}
	}
	if peer != nil {
		return peer.String()
	}
	return r.RemoteAddr
}
func (l *rateLimiter) wrap(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		host := l.clientIP(r)
		now := time.Now()
		l.mu.Lock()
		entry := l.entries[host]
		if now.After(entry.reset) {
			entry = rateEntry{reset: now.Add(l.window)}
		}
		entry.count++
		l.entries[host] = entry
		allowed := entry.count <= l.limit
		if len(l.entries) > 10000 {
			for key, value := range l.entries {
				if now.After(value.reset) {
					delete(l.entries, key)
				}
			}
		}
		l.mu.Unlock()
		if !allowed {
			w.Header().Set("Retry-After", "60")
			problem(w, 429, "rate_limited", "Too many requests. Try again shortly.")
			return
		}
		next(w, r)
	}
}
