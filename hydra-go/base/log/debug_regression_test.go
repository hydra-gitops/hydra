package log

import (
	"bytes"
	"log/slog"
	"strings"
	"testing"
)

// Helm v4 logs symlink traversal via slog.Info in internal/sympath while Hydra wraps
// helm template in [WithoutDebug0]. A previous implementation of disableDebugLogging
// replaced [slog.Default] with a plain text handler, dropping the configured Hydra chain.
func TestWithoutDebug_preservesConfiguredSlogHandler(t *testing.T) {
	var buf bytes.Buffer
	pb := NewDummyProgressBarsTo(&buf)
	Configure(Config{
		Level:        LevelInfo,
		Json:         true,
		ProgressBars: pb,
	})
	t.Cleanup(func() {
		Configure(Config{Level: LevelInfo})
	})

	WithoutDebug0(func() {
		slog.Info("helm-side-message", slog.String("path", "/chart"))
	})

	out := buf.String()
	if !strings.Contains(out, "helm-side-message") {
		t.Fatalf("expected slog.Info to use Hydra-configured handler; got %q", out)
	}
	// JSON handler from Configure — not the bare slog text handler ("time=… level=INFO …")
	if !strings.HasPrefix(strings.TrimSpace(out), "{") {
		t.Fatalf("expected JSON log line, got %q", out)
	}
}
