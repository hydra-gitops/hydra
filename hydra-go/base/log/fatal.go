package log

import (
	"context"
	"log/slog"
	"os"
)

// Fatal logs an error message and exits with code 1
func Fatal(msg string, args ...any) {
	ctx := WithNoSource(context.Background())
	slog.ErrorContext(ctx, msg, args...)
	os.Exit(1)
}
