package app

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func env(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func Healthcheck() error {
	client := http.Client{Timeout: 2 * time.Second}
	r, err := client.Get("http://127.0.0.1" + env("KOALAPARTY_ADDR", ":8080") + "/api/health")
	if err != nil {
		return err
	}
	defer r.Body.Close()
	if r.StatusCode != http.StatusOK {
		return fmt.Errorf("health returned %s", r.Status)
	}
	return nil
}

func Run() error {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/health", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"status":"ok"}`)
	})
	mux.HandleFunc("GET /api/ready", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"status":"ready"}`)
	})
	mux.Handle("/", http.FileServer(http.Dir(env("KOALAPARTY_WEB_ROOT", "../frontend/build"))))
	srv := &http.Server{Addr: env("KOALAPARTY_ADDR", ":8080"), Handler: securityHeaders(mux), ReadHeaderTimeout: 5 * time.Second, IdleTimeout: 60 * time.Second}
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-stop
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = srv.Shutdown(ctx)
	}()
	fmt.Printf(`{"level":"info","message":"server started","addr":%q}`+"\n", srv.Addr)
	err := srv.ListenAndServe()
	if err == http.ErrServerClosed {
		return nil
	}
	return err
}

func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("Permissions-Policy", "camera=(), microphone=(), geolocation=(), payment=(), usb=()")
		w.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'self' https://www.youtube.com https://www.youtube-nocookie.com; frame-src https://www.youtube-nocookie.com; img-src 'self' data: https://i.ytimg.com; connect-src 'self' ws: wss: https://www.youtube.com; style-src 'self' 'unsafe-inline'; object-src 'none'; base-uri 'self'; frame-ancestors 'none'")
		next.ServeHTTP(w, r)
	})
}
