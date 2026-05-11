package api

import (
	"encoding/json"
	"net/http"
	"os"
	"strings"
	"time"

	logrus "github.com/sirupsen/logrus"
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
		origin = "http://localhost:3000"
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

		duration := time.Since(start)
		fields := logrus.Fields{
			"method":   r.Method,
			"path":     r.URL.Path,
			"status":   rw.status,
			"duration": duration.String(),
		}

		if logrus.IsLevelEnabled(logrus.DebugLevel) {
			fields["remote_addr"] = r.RemoteAddr
			fields["user_agent"] = r.UserAgent()
			fields["query"] = r.URL.RawQuery
			fields["bytes_written"] = rw.bytesWritten
		}

		entry := logrus.WithFields(fields)
		if rw.status >= 500 {
			entry.Error("HTTP request")
		} else if rw.status >= 400 {
			entry.Warn("HTTP request")
		} else {
			entry.Info("HTTP request")
		}
	})
}

// ErrorMiddleware wraps panics and unhandled errors into ErrorWithCode JSON.
func ErrorMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rec := recover(); rec != nil {
				logrus.WithField("panic", rec).Error("Recovered from panic in HTTP handler")
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

// ConfigureLogging sets up logrus with JSON formatting and the log level
// from the LOG_LEVEL environment variable.
func ConfigureLogging() {
	logrus.SetFormatter(&logrus.JSONFormatter{
		TimestampFormat: time.RFC3339,
	})

	level := os.Getenv("LOG_LEVEL")
	if level == "" {
		level = "info"
	}

	parsed, err := logrus.ParseLevel(strings.ToLower(level))
	if err != nil {
		logrus.WithField("level", level).Warn("Invalid LOG_LEVEL — defaulting to info")
		parsed = logrus.InfoLevel
	}

	logrus.SetLevel(parsed)
	logrus.WithField("level", parsed.String()).Info("Logging configured")
}
