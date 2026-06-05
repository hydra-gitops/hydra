package log

import (
	"context"
	"log/slog"
	"strings"
)

// transformerHandler wraps a handler and transforms log records
type transformerHandler struct {
	handler slog.Handler
	debug   bool
}

func (h *transformerHandler) Handle(ctx context.Context, record slog.Record) error {
	// If message starts with "found symbolic link in path" and level is Info, convert to Debug
	if record.Level == slog.LevelInfo && strings.HasPrefix(record.Message, "found symbolic link in path") {
		// Only log as debug if debug mode is enabled
		if !h.debug {
			return nil
		}
		record.Level = slog.LevelDebug
	}
	return h.handler.Handle(ctx, record)
}

func (h *transformerHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &transformerHandler{handler: h.handler.WithAttrs(attrs), debug: h.debug}
}

func (h *transformerHandler) WithGroup(name string) slog.Handler {
	return &transformerHandler{handler: h.handler.WithGroup(name), debug: h.debug}
}

func (h *transformerHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.handler.Enabled(ctx, level)
}
