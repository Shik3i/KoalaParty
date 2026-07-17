package app

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"regexp"
	"strings"
	"syscall"
	"time"

	"github.com/Shik3i/KoalaParty/backend/internal/database"
)

type application struct {
	db                *sql.DB
	hub               *hub
	sessionTTL        time.Duration
	cookieSecure      bool
	trustedOrigins    map[string]bool
	activityMaxAge    time.Duration
	activityMaxEvents int
	roomMaxIdle       time.Duration
}

func env(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
func Healthcheck() error {
	c := http.Client{Timeout: 2 * time.Second}
	r, e := c.Get("http://127.0.0.1" + env("KOALAPARTY_ADDR", ":8080") + "/api/health")
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
	db, e := database.Open(env("KOALAPARTY_DB", "koalaparty.db"))
	if e != nil {
		return e
	}
	defer db.Close()
	ttl, _ := time.ParseDuration(env("KOALAPARTY_SESSION_TTL", "168h"))
	if ttl == 0 {
		ttl = 168 * time.Hour
	}
	a := &application{db: db, hub: newHub(), sessionTTL: ttl, cookieSecure: env("KOALAPARTY_COOKIE_SECURE", "false") == "true", trustedOrigins: map[string]bool{}}
	a.activityMaxAge, _ = time.ParseDuration(env("KOALAPARTY_ACTIVITY_MAX_AGE", "720h"))
	if a.activityMaxAge == 0 {
		a.activityMaxAge = 720 * time.Hour
	}
	a.roomMaxIdle, _ = time.ParseDuration(env("KOALAPARTY_ROOM_MAX_IDLE", "8760h"))
	if a.roomMaxIdle == 0 {
		a.roomMaxIdle = 8760 * time.Hour
	}
	if _, e := fmt.Sscanf(env("KOALAPARTY_ACTIVITY_MAX_EVENTS", "200"), "%d", &a.activityMaxEvents); e != nil || a.activityMaxEvents < 10 {
		a.activityMaxEvents = 200
	}
	for _, o := range strings.Split(env("KOALAPARTY_TRUSTED_ORIGINS", "http://localhost:5173,http://localhost:8080"), ",") {
		a.trustedOrigins[strings.TrimSpace(o)] = true
	}
	mux := http.NewServeMux()
	authLimiter := newRateLimiter(20, time.Minute)
	commandLimiter := newRateLimiter(180, time.Minute)
	mux.HandleFunc("GET /api/health", func(w http.ResponseWriter, _ *http.Request) { writeJSON(w, 200, map[string]string{"status": "ok"}) })
	mux.HandleFunc("GET /api/ready", func(w http.ResponseWriter, _ *http.Request) {
		if e := db.Ping(); e != nil {
			problem(w, 503, "not_ready", "Database unavailable.")
			return
		}
		writeJSON(w, 200, map[string]string{"status": "ready"})
	})
	mux.HandleFunc("POST /api/identity/exchange", authLimiter.wrap(a.exchangeIdentity))
	mux.HandleFunc("GET /api/me", a.requireAuth(a.me))
	mux.HandleFunc("POST /api/accounts/register", a.requireAuth(a.register))
	mux.HandleFunc("POST /api/accounts/login", authLimiter.wrap(a.login))
	mux.HandleFunc("POST /api/accounts/logout", a.requireAuth(a.logout))
	mux.HandleFunc("GET /api/friends", a.requireAuth(a.friends))
	mux.HandleFunc("POST /api/friends", a.requireAuth(a.friends))
	mux.HandleFunc("POST /api/friends/{username}/{action}", a.requireAuth(a.friendAction))
	mux.HandleFunc("POST /api/rooms", a.requireAuth(a.createRoom))
	mux.HandleFunc("GET /api/rooms/{roomId}", a.requireAuth(a.roomSnapshot))
	mux.HandleFunc("POST /api/rooms/{roomId}/commands", commandLimiter.wrap(a.requireAuth(a.roomCommand)))
	mux.HandleFunc("GET /api/rooms/{roomId}/ws", a.requireAuth(a.websocket))
	mux.HandleFunc("POST /api/rooms/{roomId}/reports", a.requireAuth(a.report))
	mux.HandleFunc("GET /api/discover", a.discover)
	webRoot := env("KOALAPARTY_WEB_ROOT", "../frontend/build")
	mux.Handle("/", spaHandler(webRoot))
	go a.maintenanceLoop(context.Background())
	srv := &http.Server{Addr: env("KOALAPARTY_ADDR", ":8080"), Handler: securityHeaders(mux, contentSecurityPolicy(webRoot)), ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 15 * time.Second, WriteTimeout: 30 * time.Second, IdleTimeout: 60 * time.Second}
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-stop
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = srv.Shutdown(ctx)
	}()
	fmt.Printf(`{"level":"info","message":"server started","addr":%q}`+"\n", srv.Addr)
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
