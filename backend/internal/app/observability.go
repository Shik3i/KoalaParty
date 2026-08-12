package app

import (
	"bufio"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"os"
	"regexp"
	"strings"
	"sync/atomic"
	"time"
)

type requestIDContextKey struct{}

var roomPathPattern = regexp.MustCompile(`/rooms/[A-Z2-7]{16}`)

type runtimeMetrics struct {
	httpRequests       atomic.Uint64
	httpErrors         atomic.Uint64
	commandsAccepted   atomic.Uint64
	commandsRejected   atomic.Uint64
	websocketOpened    atomic.Uint64
	websocketClosed    atomic.Uint64
	websocketQueueFull atomic.Uint64
}

func newRuntimeMetrics() *runtimeMetrics { return &runtimeMetrics{} }

func (m *runtimeMetrics) snapshot() map[string]uint64 {
	if m == nil {
		return map[string]uint64{}
	}
	return map[string]uint64{
		"http_requests_total":        m.httpRequests.Load(),
		"http_errors_total":          m.httpErrors.Load(),
		"commands_accepted_total":    m.commandsAccepted.Load(),
		"commands_rejected_total":    m.commandsRejected.Load(),
		"websocket_opened_total":     m.websocketOpened.Load(),
		"websocket_closed_total":     m.websocketClosed.Load(),
		"websocket_queue_full_total": m.websocketQueueFull.Load(),
	}
}

func newLogger() *slog.Logger {
	level := slog.LevelInfo
	switch strings.ToLower(strings.TrimSpace(env("KOALAPARTY_LOG_LEVEL", "info"))) {
	case "debug":
		level = slog.LevelDebug
	case "warn", "warning":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	}
	options := &slog.HandlerOptions{Level: level, AddSource: level <= slog.LevelDebug}
	if strings.EqualFold(strings.TrimSpace(env("KOALAPARTY_LOG_FORMAT", "json")), "text") {
		return slog.New(slog.NewTextHandler(os.Stdout, options))
	}
	return slog.New(slog.NewJSONHandler(os.Stdout, options))
}

func loggerWithWriter(logger *slog.Logger) *slog.Logger {
	if logger != nil {
		return logger
	}
	return slog.New(slog.NewJSONHandler(io.Discard, nil))
}

func requestID(r *http.Request) string {
	if r == nil {
		return ""
	}
	if id := strings.TrimSpace(r.Header.Get("X-Request-ID")); validRequestID(id) {
		return id
	}
	return newRequestID()
}

func newRequestID() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return fmt.Sprintf("req-%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(b[:])
}

func validRequestID(value string) bool {
	if value == "" || len(value) > 128 {
		return false
	}
	for _, r := range value {
		if r < 0x21 || r > 0x7e {
			return false
		}
	}
	return true
}

func withRequestID(r *http.Request, id string) *http.Request {
	return r.WithContext(context.WithValue(r.Context(), requestIDContextKey{}, id))
}

func requestIDFromContext(ctx context.Context) string {
	if value, ok := ctx.Value(requestIDContextKey{}).(string); ok {
		return value
	}
	return ""
}

func safeRoute(path string) string {
	if path == "" {
		return "/"
	}
	return roomPathPattern.ReplaceAllString(path, "/rooms/:roomId")
}

func shortHash(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:6])
}

type statusWriter struct {
	http.ResponseWriter
	status int
	bytes  int
}

func (w *statusWriter) WriteHeader(status int) {
	if w.status != 0 {
		return
	}
	w.status = status
	w.ResponseWriter.WriteHeader(status)
}

func (w *statusWriter) Write(p []byte) (int, error) {
	if w.status == 0 {
		w.WriteHeader(http.StatusOK)
	}
	n, err := w.ResponseWriter.Write(p)
	w.bytes += n
	return n, err
}

func (w *statusWriter) Unwrap() http.ResponseWriter { return w.ResponseWriter }

func (w *statusWriter) Flush() {
	if w.status == 0 {
		w.WriteHeader(http.StatusOK)
	}
	if flusher, ok := w.ResponseWriter.(http.Flusher); ok {
		flusher.Flush()
	}
}

func (w *statusWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	if w.status == 0 {
		w.status = http.StatusSwitchingProtocols
	}
	hijacker, ok := w.ResponseWriter.(http.Hijacker)
	if !ok {
		return nil, nil, fmt.Errorf("response writer does not support hijacking")
	}
	return hijacker.Hijack()
}

func requestLogging(logger *slog.Logger, metrics *runtimeMetrics, next http.Handler) http.Handler {
	logger = loggerWithWriter(logger)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := requestID(r)
		r = withRequestID(r, id)
		w.Header().Set("X-Request-ID", id)
		started := time.Now()
		recorder := &statusWriter{ResponseWriter: w}
		next.ServeHTTP(recorder, r)
		status := recorder.status
		if status == 0 {
			status = http.StatusOK
		}
		if metrics != nil {
			metrics.httpRequests.Add(1)
			if status >= 500 {
				metrics.httpErrors.Add(1)
			}
		}
		level := slog.LevelInfo
		if status >= 500 {
			level = slog.LevelError
		} else if status >= 400 {
			level = slog.LevelWarn
		}
		logger.LogAttrs(r.Context(), level, "http request",
			slog.String("request_id", id),
			slog.String("method", r.Method),
			slog.String("route", safeRoute(r.URL.Path)),
			slog.Int("status", status),
			slog.Int("bytes", recorder.bytes),
			slog.Int64("duration_ms", time.Since(started).Milliseconds()),
		)
	})
}

func (a *application) adminMetrics(w http.ResponseWriter, _ *http.Request, _ principal) {
	writeJSON(w, http.StatusOK, a.metrics.snapshot())
}

func (a *application) logCommand(ctx context.Context, room string, p principal, c command, outcome, code string) {
	if a == nil || a.logger == nil {
		return
	}
	attrs := []slog.Attr{
		slog.String("request_id", requestIDFromContext(ctx)),
		slog.String("command_request_hash", shortHash(c.RequestID)),
		slog.String("command", c.Type),
		slog.String("room_hash", shortHash(room)),
		slog.String("identity_hash", shortHash(p.IdentityID)),
		slog.String("outcome", outcome),
	}
	if code != "" {
		attrs = append(attrs, slog.String("error_code", code))
	}
	a.logger.LogAttrs(ctx, slog.LevelInfo, "room command", attrs...)
}

func commandErrorCode(err error) string {
	if err == nil {
		return ""
	}
	if errors.Is(err, errDenied) {
		return "permission_denied"
	}
	if errors.Is(err, errStale) {
		return "stale_revision"
	}
	message := strings.ToLower(err.Error())
	switch {
	case strings.HasPrefix(message, "invalid"):
		return "invalid_command"
	case strings.Contains(message, "already used"):
		return "request_id_conflict"
	case strings.Contains(message, "unknown"):
		return "not_found"
	case strings.HasPrefix(message, "unsupported"):
		return "unsupported_command"
	default:
		return "command_failed"
	}
}
