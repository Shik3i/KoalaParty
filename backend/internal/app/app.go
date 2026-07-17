package app

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/Shik3i/KoalaParty/backend/internal/database"
)

type application struct {
	db             *sql.DB
	hub            *hub
	sessionTTL     time.Duration
	cookieSecure   bool
	trustedOrigins map[string]bool
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
	for _, o := range strings.Split(env("KOALAPARTY_TRUSTED_ORIGINS", "http://localhost:5173,http://localhost:8080"), ",") {
		a.trustedOrigins[strings.TrimSpace(o)] = true
	}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/health", func(w http.ResponseWriter, _ *http.Request) { writeJSON(w, 200, map[string]string{"status": "ok"}) })
	mux.HandleFunc("GET /api/ready", func(w http.ResponseWriter, _ *http.Request) {
		if e := db.Ping(); e != nil {
			problem(w, 503, "not_ready", "Database unavailable.")
			return
		}
		writeJSON(w, 200, map[string]string{"status": "ready"})
	})
	mux.HandleFunc("POST /api/identity/exchange", a.exchangeIdentity)
	mux.HandleFunc("GET /api/me", a.requireAuth(a.me))
	mux.HandleFunc("POST /api/accounts/register", a.requireAuth(a.register))
	mux.HandleFunc("POST /api/accounts/login", a.login)
	mux.HandleFunc("POST /api/accounts/logout", a.requireAuth(a.logout))
	mux.HandleFunc("GET /api/friends", a.requireAuth(a.friends))
	mux.HandleFunc("POST /api/friends", a.requireAuth(a.friends))
	mux.HandleFunc("POST /api/friends/{username}/{action}", a.requireAuth(a.friendAction))
	mux.HandleFunc("POST /api/rooms", a.requireAuth(a.createRoom))
	mux.HandleFunc("GET /api/rooms/{roomId}", a.requireAuth(a.roomSnapshot))
	mux.HandleFunc("POST /api/rooms/{roomId}/commands", a.requireAuth(a.roomCommand))
	mux.HandleFunc("GET /api/rooms/{roomId}/ws", a.requireAuth(a.websocket))
	mux.HandleFunc("POST /api/rooms/{roomId}/reports", a.requireAuth(a.report))
	mux.HandleFunc("GET /api/discover", a.discover)
	mux.Handle("/", http.FileServer(http.Dir(env("KOALAPARTY_WEB_ROOT", "../frontend/build"))))
	srv := &http.Server{Addr: env("KOALAPARTY_ADDR", ":8080"), Handler: securityHeaders(mux), ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 15 * time.Second, WriteTimeout: 30 * time.Second, IdleTimeout: 60 * time.Second}
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
func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("Permissions-Policy", "camera=(), microphone=(), geolocation=(), payment=(), usb=()")
		w.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'self' https://www.youtube.com https://www.youtube-nocookie.com; frame-src https://www.youtube-nocookie.com; img-src 'self' data: https://i.ytimg.com; connect-src 'self' ws: wss: https://www.youtube.com; style-src 'self' 'unsafe-inline'; object-src 'none'; base-uri 'self'; frame-ancestors 'none'")
		next.ServeHTTP(w, r)
	})
}
