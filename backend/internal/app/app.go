package app

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/Shik3i/KoalaParty/backend/internal/database"
)

type application struct {
	db             *sql.DB
	hub            *hub
	cookieSecure   bool
	trustedOrigins map[string]bool
	trustedProxies []*net.IPNet

	// fetchTitle resolves a human-readable YouTube title for a video ID. It is
	// nil when metadata lookups are disabled (and in tests) so the command path
	// makes no outbound requests.
	fetchTitle func(ctx context.Context, videoID string) string

	// fetchSegments resolves the SponsorBlock skip segments for a video ID (cached).
	// It is nil when SponsorBlock is disabled (and in tests) so no outbound requests
	// are made. Segment data is CC BY-NC-SA 4.0 and requires attribution when shown.
	fetchSegments func(ctx context.Context, videoID string) []sponsorSegment

	mu                sync.RWMutex
	sessionTTL        time.Duration
	activityMaxAge    time.Duration
	activityMaxEvents int
	roomMaxIdle       time.Duration
	publicRooms       bool
}

func env(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
func Healthcheck() error {
	c := http.Client{Timeout: 2 * time.Second}
	addr := env("KOALAPARTY_ADDR", ":8080")
	if strings.HasPrefix(addr, ":") {
		addr = "127.0.0.1" + addr
	} else if strings.HasPrefix(addr, "0.0.0.0:") {
		addr = "127.0.0.1:" + strings.TrimPrefix(addr, "0.0.0.0:")
	}
	r, e := c.Get("http://" + addr + "/api/health")
	if e != nil {
		return e
	}
	defer r.Body.Close()
	if r.StatusCode != 200 {
		return fmt.Errorf("health returned %s", r.Status)
	}
	return nil
}

func Run() error {
	cfg, e := loadConfig()
	if e != nil {
		return fmt.Errorf("configuration: %w", e)
	}
	db, e := database.Open(cfg.dbPath)
	if e != nil {
		return e
	}
	defer db.Close()
	a := &application{db: db, hub: newHub(), sessionTTL: cfg.sessionTTL, cookieSecure: cfg.cookieSecure, trustedOrigins: cfg.trustedOrigins, trustedProxies: cfg.trustedProxies, activityMaxAge: cfg.activityMaxAge, activityMaxEvents: cfg.activityMaxEvents, roomMaxIdle: cfg.roomMaxIdle, publicRooms: cfg.publicRooms}
	if cfg.youtubeMetadata {
		a.fetchTitle = fetchYouTubeTitle
	}
	if cfg.sponsorBlock {
		a.fetchSegments = newSegmentCache(fetchSponsorSegments).get
	}
	if err := a.loadSettingsFromDB(); err != nil {
		return fmt.Errorf("load db settings: %w", err)
	}
	mux := http.NewServeMux()
	authLimiter := newRateLimiter(20, time.Minute, a.trustedProxies)
	commandLimiter := newRateLimiter(180, time.Minute, a.trustedProxies)
	mux.HandleFunc("GET /api/health", func(w http.ResponseWriter, _ *http.Request) {
		info := CurrentBuildInformation()
		writeJSON(w, 200, map[string]string{"status": "ok", "version": info.Version})
	})
	mux.HandleFunc("GET /api/version", func(w http.ResponseWriter, _ *http.Request) { writeJSON(w, 200, CurrentBuildInformation()) })
	mux.HandleFunc("GET /api/ready", func(w http.ResponseWriter, _ *http.Request) {
		if e := db.Ping(); e != nil {
			problem(w, 503, "not_ready", "Database unavailable.")
			return
		}
		writeJSON(w, 200, map[string]string{"status": "ready"})
	})
	mux.HandleFunc("POST /api/identity/exchange", authLimiter.wrap(a.exchangeIdentity))
	mux.HandleFunc("GET /api/me", a.me)
	mux.HandleFunc("POST /api/accounts/register", a.requireAuth(a.register))
	mux.HandleFunc("POST /api/accounts/login", authLimiter.wrap(a.login))
	mux.HandleFunc("POST /api/accounts/logout", a.requireAuth(a.logout))
	mux.HandleFunc("PATCH /api/account/profile", a.requireAuth(a.accountProfile))
	mux.HandleFunc("POST /api/account/password", a.requireAuth(a.accountPassword))
	mux.HandleFunc("GET /api/account/sessions", a.requireAuth(a.accountSessions))
	mux.HandleFunc("DELETE /api/account/sessions", a.requireAuth(a.accountSessions))
	mux.HandleFunc("DELETE /api/account/sessions/{sessionId}", a.requireAuth(a.revokeSession))
	mux.HandleFunc("DELETE /api/account", a.requireAuth(a.deleteOwnAccount))
	mux.HandleFunc("GET /api/friends", a.requireAuth(a.friends))
	mux.HandleFunc("POST /api/friends", a.requireAuth(a.friends))
	mux.HandleFunc("POST /api/friends/{username}/{action}", a.requireAuth(a.friendAction))
	mux.HandleFunc("POST /api/rooms", a.requireAuth(a.createRoom))
	mux.HandleFunc("GET /api/rooms", a.requireAuth(a.myRooms))
	mux.HandleFunc("POST /api/rooms/previews", a.requireAuth(a.roomPreviews))
	mux.HandleFunc("GET /api/rooms/{roomId}", a.requireAuth(a.roomSnapshot))
	mux.HandleFunc("DELETE /api/rooms/{roomId}", a.requireAuth(a.deleteRoom))
	mux.HandleFunc("DELETE /api/rooms/{roomId}/membership", a.requireAuth(a.leaveRoom))
	mux.HandleFunc("GET /api/rooms/{roomId}/invites", a.requireAuth(a.roomInvites))
	mux.HandleFunc("POST /api/rooms/{roomId}/invites", a.requireAuth(a.roomInvites))
	mux.HandleFunc("DELETE /api/rooms/{roomId}/invites/{username}", a.requireAuth(a.revokeInvite))
	mux.HandleFunc("POST /api/rooms/{roomId}/commands", commandLimiter.wrap(a.requireAuth(a.roomCommand)))
	mux.HandleFunc("GET /api/rooms/{roomId}/ws", a.requireAuth(a.websocket))
	mux.HandleFunc("POST /api/rooms/{roomId}/reports", a.requireAuth(a.report))
	mux.HandleFunc("GET /api/discover", a.discover)
	mux.HandleFunc("GET /api/admin/stats", a.requireAdmin(a.adminStats))
	mux.HandleFunc("GET /api/admin/settings", a.requireAdmin(a.adminSettings))
	mux.HandleFunc("POST /api/admin/settings", a.requireAdmin(a.adminSettings))
	mux.HandleFunc("GET /api/admin/reports", a.requireAdmin(a.adminReports))
	mux.HandleFunc("POST /api/admin/reports/{reportId}/resolve", a.requireAdmin(a.resolveReport))
	mux.HandleFunc("POST /api/admin/reports/{reportId}/delist", a.requireAdmin(a.delistReport))
	webRoot := cfg.webRoot
	mux.Handle("/", spaHandler(webRoot))
	maintenanceCtx, cancelMaintenance := context.WithCancel(context.Background())
	defer cancelMaintenance()
	go a.maintenanceLoop(maintenanceCtx)
	srv := &http.Server{Addr: cfg.addr, Handler: securityHeaders(mux, contentSecurityPolicy(webRoot)), ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 15 * time.Second, WriteTimeout: 30 * time.Second, IdleTimeout: 60 * time.Second}
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-stop
		cancelMaintenance()
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = srv.Shutdown(ctx)
	}()
	fmt.Printf(`{"level":"info","message":"server started","addr":%q,"version":%q,"commit":%q}`+"\n", srv.Addr, Version, Commit)
	e = srv.ListenAndServe()
	if e == http.ErrServerClosed {
		return nil
	}
	return e
}
func (a *application) originAllowed(o string) bool { return o != "" && a.trustedOrigins[o] }
func spaHandler(root string) http.Handler {
	files := http.FileServer(http.Dir(root))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/api/") {
			problem(w, 404, "not_found", "API route was not found.")
			return
		}
		target := filepath.Join(root, filepath.Clean(r.URL.Path))
		if info, e := os.Stat(target); e == nil && !info.IsDir() {
			files.ServeHTTP(w, r)
			return
		}
		clone := r.Clone(r.Context())
		clone.URL.Path = "/"
		files.ServeHTTP(w, clone)
	})
}
func contentSecurityPolicy(root string) string {
	scriptSrc := "'self' https://www.youtube.com https://www.youtube-nocookie.com"
	if body, e := os.ReadFile(filepath.Join(root, "index.html")); e == nil {
		re := regexp.MustCompile(`(?s)<script[^>]*>(.*?)</script>`)
		for _, match := range re.FindAllSubmatch(body, -1) {
			if len(match[1]) > 0 {
				sum := sha256.Sum256(match[1])
				scriptSrc += " 'sha256-" + base64.StdEncoding.EncodeToString(sum[:]) + "'"
			}
		}
	}
	return "default-src 'self'; script-src " + scriptSrc + "; frame-src https://www.youtube-nocookie.com; img-src 'self' data: https://i.ytimg.com; connect-src 'self' ws: wss: https://www.youtube.com; style-src 'self' 'unsafe-inline'; object-src 'none'; base-uri 'self'; frame-ancestors 'none'"
}
func securityHeaders(next http.Handler, csp string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("Permissions-Policy", "camera=(), microphone=(), geolocation=(), payment=(), usb=()")
		w.Header().Set("Content-Security-Policy", csp)
		next.ServeHTTP(w, r)
	})
}

func (a *application) requireAdmin(next func(http.ResponseWriter, *http.Request, principal)) http.HandlerFunc {
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
		if p.AccountID == "" {
			problem(w, 403, "admin_required", "Administrator privileges required.")
			return
		}
		var isAdmin int
		err := a.db.QueryRow("SELECT is_admin FROM accounts WHERE id=?", p.AccountID).Scan(&isAdmin)
		if err != nil || isAdmin != 1 {
			problem(w, 403, "admin_required", "Administrator privileges required.")
			return
		}
		next(w, r, p)
	}
}

func (a *application) getSessionTTL() time.Duration {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.sessionTTL
}

func (a *application) setSessionTTL(d time.Duration) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.sessionTTL = d
}

func (a *application) getActivityMaxAge() time.Duration {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.activityMaxAge
}

func (a *application) setActivityMaxAge(d time.Duration) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.activityMaxAge = d
}

func (a *application) getActivityMaxEvents() int {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.activityMaxEvents
}

func (a *application) setActivityMaxEvents(n int) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.activityMaxEvents = n
}

func (a *application) getRoomMaxIdle() time.Duration {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.roomMaxIdle
}

func (a *application) setRoomMaxIdle(d time.Duration) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.roomMaxIdle = d
}

func (a *application) getPublicRooms() bool {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.publicRooms
}

func (a *application) setPublicRooms(b bool) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.publicRooms = b
}

func (a *application) loadSettingsFromDB() error {
	a.mu.Lock()
	defer a.mu.Unlock()

	rows, err := a.db.Query("SELECT key, value FROM settings")
	if err != nil {
		return nil
	}
	defer rows.Close()

	for rows.Next() {
		var key, val string
		if err := rows.Scan(&key, &val); err != nil {
			return err
		}
		switch key {
		case "session_ttl":
			d, err := time.ParseDuration(val)
			if err == nil {
				a.sessionTTL = d
			}
		case "activity_max_age":
			d, err := time.ParseDuration(val)
			if err == nil {
				a.activityMaxAge = d
			}
		case "activity_max_events":
			n, err := strconv.Atoi(val)
			if err == nil {
				a.activityMaxEvents = n
			}
		case "room_max_idle":
			d, err := time.ParseDuration(val)
			if err == nil {
				a.roomMaxIdle = d
			}
		case "public_rooms":
			b, err := strconv.ParseBool(val)
			if err == nil {
				a.publicRooms = b
			}
		}
	}
	return rows.Err()
}
