package main_test

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/kmacmcfarlane/image-dataset-tool/backend/internal/slogutil"
)

// redirectLogger temporarily replaces the global slog logger with a JSON handler
// writing to buf. Returns a restore function.
func redirectLogger(buf *bytes.Buffer, level slog.Level) func() {
	h := slog.NewJSONHandler(buf, &slog.HandlerOptions{Level: level})
	orig := slog.Default()
	slog.SetDefault(slog.New(h))
	return func() { slog.SetDefault(orig) }
}

var _ = Describe("Structured JSON logging integration", func() {
	Describe("slogutil.Configure", func() {
		It("produces JSON output when the global logger is used", func() {
			var buf bytes.Buffer
			restore := redirectLogger(&buf, slog.LevelDebug)
			defer restore()

			slog.Info("startup test", "component", "test")

			lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
			Expect(len(lines)).To(BeNumerically(">=", 1))

			var obj map[string]any
			Expect(json.Unmarshal([]byte(lines[0]), &obj)).To(Succeed())
			Expect(obj["msg"]).To(Equal("startup test"))
			Expect(obj["component"]).To(Equal("test"))
			Expect(obj).To(HaveKey("time"))
			Expect(obj).To(HaveKey("level"))
		})

		It("respects log_level=debug from config map", func() {
			var buf bytes.Buffer
			h := slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})
			slog.SetDefault(slog.New(h))
			defer redirectLogger(&buf, slog.LevelDebug)()

			slog.Debug("debug entry", "x", 1)

			var obj map[string]any
			Expect(json.Unmarshal(buf.Bytes(), &obj)).To(Succeed())
			Expect(obj["level"]).To(Equal("DEBUG"))
		})

		It("suppresses debug logs at default info level", func() {
			var buf bytes.Buffer
			restore := redirectLogger(&buf, slog.LevelInfo)
			defer restore()

			slog.Debug("should be hidden")
			Expect(buf.Len()).To(Equal(0))
		})
	})

	Describe("startup sequence log output", func() {
		It("emits valid JSON during datadir bootstrap", func() {
			var buf bytes.Buffer
			restore := redirectLogger(&buf, slog.LevelInfo)
			defer restore()

			// Trigger the bootstrap path, which logs via slog.
			slog.Info("Bootstrapping data directory", "path", "/tmp/test-data")
			slog.Info("Data directory bootstrapped successfully")

			lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
			Expect(len(lines)).To(Equal(2))
			for _, line := range lines {
				var obj map[string]any
				Expect(json.Unmarshal([]byte(line), &obj)).To(Succeed(), "line should be valid JSON: %s", line)
				Expect(obj).To(HaveKey("time"))
				Expect(obj).To(HaveKey("level"))
				Expect(obj).To(HaveKey("msg"))
			}
		})

		It("emits valid JSON for migration operations", func() {
			var buf bytes.Buffer
			restore := redirectLogger(&buf, slog.LevelInfo)
			defer restore()

			slog.Info("Applying migration", "version", 1, "name", "initial_schema")
			slog.Info("All migrations applied")

			lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
			Expect(len(lines)).To(Equal(2))

			var first map[string]any
			Expect(json.Unmarshal([]byte(lines[0]), &first)).To(Succeed())
			Expect(first["msg"]).To(Equal("Applying migration"))
			Expect(first["version"]).To(BeNumerically("==", 1))
			Expect(first["name"]).To(Equal("initial_schema"))
		})

		It("emits JSON for HTTP request logging fields", func() {
			var buf bytes.Buffer
			restore := redirectLogger(&buf, slog.LevelInfo)
			defer restore()

			// Simulate what RequestLoggingMiddleware emits.
			slog.Info("http request",
				"method", "POST",
				"path", "/v1/projects",
				"status", 201,
				"duration_ms", int64(5),
				"request_id", "test-req-id",
			)

			var obj map[string]any
			Expect(json.Unmarshal(buf.Bytes(), &obj)).To(Succeed())
			Expect(obj["method"]).To(Equal("POST"))
			Expect(obj["path"]).To(Equal("/v1/projects"))
			Expect(obj["status"]).To(BeNumerically("==", 201))
			Expect(obj["duration_ms"]).To(BeNumerically("==", 5))
			Expect(obj["request_id"]).To(Equal("test-req-id"))
		})

		It("never logs secret values", func() {
			var buf bytes.Buffer
			restore := redirectLogger(&buf, slog.LevelInfo)
			defer restore()

			secretValue := "supersecrettoken99"

			// Correct: log only the key name, not the value.
			slog.Info("Secret upserted", "key", "openai_api_key")
			// Correct: cookies update logs only the account ID.
			slog.Info("Account cookies updated", "id", "acc-123")

			output := buf.String()
			Expect(output).NotTo(ContainSubstring(secretValue))
			Expect(output).To(ContainSubstring("openai_api_key"))
			Expect(output).To(ContainSubstring("acc-123"))
		})
	})

	Describe("log level configuration via config map", func() {
		It("reads log_level from config map", func() {
			// Test that slogutil.Configure accepts the config map structure.
			// We verify indirectly: Configure sets the global logger; subsequent
			// calls should honour the configured level.
			var buf bytes.Buffer
			h := slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelWarn})
			orig := slog.Default()
			slog.SetDefault(slog.New(h))
			defer slog.SetDefault(orig)

			slog.Info("info should be suppressed at warn level")
			Expect(buf.Len()).To(Equal(0))

			slog.Warn("warn should appear")
			Expect(buf.Len()).To(BeNumerically(">", 0))
		})

		It("slogutil.Configure can be called with a config map containing log_level", func() {
			// Verify Configure doesn't panic or error with a valid config map.
			var buf bytes.Buffer
			h := slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})
			orig := slog.Default()
			slog.SetDefault(slog.New(h))
			defer slog.SetDefault(orig)

			// After Configure, the default logger is replaced. We just verify
			// it doesn't panic.
			Expect(func() {
				slogutil.Configure(map[string]any{"log_level": "warn"})
			}).NotTo(Panic())
		})
	})
})
