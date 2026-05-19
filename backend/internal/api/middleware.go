package api

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"os"
	"time"
)

// ErrorWithCode is the standard JSON error response shape.
type ErrorWithCode struct {
	Error string `json:"error"`
	Code  int    `json:"code"`
}

// CORSMiddleware adds CORS headers for the frontend dev origin.
func CORSMiddleware(next http.Handler) http.Handler {
	origin := os.Getenv("CORS_ORIGIN")
	if origin == "" {
		origin = "http://localhost:3002"
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", origin)
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, Last-Event-ID")
		w.Header().Set("Access-Control-Allow-Credentials", "true")
		w.Header().Set("Access-Control-Max-Age", "86400")

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// RequestLoggingMiddleware logs every HTTP request with structured fields.
// At debug level, full request/response details are included.
func RequestLoggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		// Wrap response writer to capture status code.
		rw := &responseRecorder{ResponseWriter: w, status: http.StatusOK}

		next.ServeHTTP(rw, r)

		durationMs := time.Since(start).Milliseconds()

		attrs := []any{
			"method",      r.Method,
			"path",        r.URL.Path,
			"status",      rw.status,
			"duration_ms", durationMs,
			"request_id",  r.Header.Get("X-Request-Id"),
		}

		if slog.Default().Enabled(r.Context(), slog.LevelDebug) {
			attrs = append(attrs,
				"remote_addr",    r.RemoteAddr,
				"user_agent",     r.UserAgent(),
				"query",          r.URL.RawQuery,
				"bytes_written",  rw.bytesWritten,
			)
		}

		msg := "http request"
		switch {
		case rw.status >= 500:
			slog.Error(msg, attrs...)
		case rw.status >= 400:
			slog.Warn(msg, attrs...)
		default:
			slog.Info(msg, attrs...)
		}
	})
}

// ErrorMiddleware wraps panics and unhandled errors into ErrorWithCode JSON.
func ErrorMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rec := recover(); rec != nil {
				slog.Error("Recovered from panic in HTTP handler", "panic", rec)
				respondError(w, http.StatusInternalServerError, "internal server error")
			}
		}()
		next.ServeHTTP(w, r)
	})
}

func respondError(w http.ResponseWriter, code int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(ErrorWithCode{Error: msg, Code: code})
}

// responseRecorder wraps http.ResponseWriter to capture status and bytes written.
type responseRecorder struct {
	http.ResponseWriter
	status       int
	bytesWritten int
	wroteHeader  bool
}

func (rw *responseRecorder) WriteHeader(code int) {
	if !rw.wroteHeader {
		rw.status = code
		rw.wroteHeader = true
		rw.ResponseWriter.WriteHeader(code)
	}
}

func (rw *responseRecorder) Write(b []byte) (int, error) {
	if !rw.wroteHeader {
		rw.WriteHeader(http.StatusOK)
	}
	n, err := rw.ResponseWriter.Write(b)
	rw.bytesWritten += n
	return n, err
}

// Flush implements http.Flusher for SSE support.
func (rw *responseRecorder) Flush() {
	if f, ok := rw.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}
