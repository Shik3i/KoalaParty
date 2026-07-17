package app

import (
	"net"
	"net/http"
	"sync"
	"time"
)

type rateEntry struct {
	count int
	reset time.Time
}
type rateLimiter struct {
	mu      sync.Mutex
	limit   int
	window  time.Duration
	entries map[string]rateEntry
}

func newRateLimiter(limit int, window time.Duration) *rateLimiter {
	return &rateLimiter{limit: limit, window: window, entries: map[string]rateEntry{}}
}
func (l *rateLimiter) wrap(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		host, _, e := net.SplitHostPort(r.RemoteAddr)
		if e != nil {
			host = r.RemoteAddr
		}
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
