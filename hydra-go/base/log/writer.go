package log

import (
	"context"
	"io"
	"log/slog"
	"strings"
)

// SlogWriter wraps writes to slog
type SlogWriter struct {
	level  slog.Level
	prefix string
}

// Write implements io.Writer interface, logging to slog
func (w *SlogWriter) Write(p []byte) (n int, err error) {
	msg := strings.TrimSpace(string(p))
	if msg != "" {
		// Use a context with no source logging
		ctx := WithNoSource(context.Background())
		slog.Log(ctx, w.level, w.prefix+msg)
	}
	return len(p), nil
}

// NewSlogWriter creates a new Writer that logs at the specified level
func NewSlogWriter(prefix string, level slog.Level) io.Writer {
	return &SlogWriter{prefix: prefix, level: level}
}
