package log

import (
	"context"
	"log/slog"
	"runtime"
	"time"
)

// Logger provides structured logging methods.
type Logger struct {
	handler slog.Handler
}

// NewProgress creates a single footer bar for operation with the given total (use total < 0 for unknown-count / spinner-style steps).
// Returns (nil, nil) when terminal progress is disabled or no ProgressBars are configured.
func (l Logger) NewProgress(operation string, total int) (Progress, error) {
	if !terminalProgressUI.Load() {
		return nil, nil
	}
	SyncStdoutBestEffort()
	pb := activeProgressBars
	if pb == nil {
		return nil, nil
	}
	return pb.NewProgress(operation, total)
}

// NewLogger creates a new Logger using the default slog handler.
func NewLogger() Logger {
	return Logger{handler: slog.Default().Handler()}
}

// NewLoggerWithHandler creates a new Logger with a custom handler.
func NewLoggerWithHandler(handler slog.Handler) Logger {
	return Logger{handler: handler}
}

// logWithCallerPC creates a slog.Record with the correct caller PC (skipping internal frames)
// and dispatches it to this logger's handler.
func (l Logger) logWithCallerPC(level slog.Level, msg string, args ...any) {
	handler := l.handler
	if handler == nil {
		handler = slog.Default().Handler()
	}
	if !handler.Enabled(context.Background(), level) {
		return
	}
	var pcs [1]uintptr
	// skip: runtime.Callers, logWithCallerPC, DebugLog/Info/Warn/Error
	runtime.Callers(3, pcs[:])
	r := slog.NewRecord(time.Now(), level, msg, pcs[0])
	r.Add(args...)
	_ = handler.Handle(context.Background(), r)
}

// DebugLog logs a debug message with a LogId and optional attributes.
func (l Logger) DebugLog(id LogId, msg string, args ...any) {
	l.logWithCallerPC(slog.LevelDebug, msg, append([]any{String("logId", id.String())}, args...)...)
}

// Info logs an info message with optional attributes (logId is omitted at INFO level).
func (l Logger) Info(id LogId, msg string, args ...any) {
	l.logWithCallerPC(slog.LevelInfo, msg, args...)
}

// Warn logs a warning message with a LogId and optional attributes.
func (l Logger) Warn(id LogId, msg string, args ...any) {
	h := l.handler
	if h == nil {
		h = slog.Default().Handler()
	}
	if h.Enabled(context.Background(), slog.LevelDebug) {
		args = append([]any{String("logId", id.String())}, args...)
	}
	l.logWithCallerPC(slog.LevelWarn, msg, args...)
}

// Error logs an error message with a LogId and optional attributes.
func (l Logger) Error(id LogId, msg string, args ...any) {
	h := l.handler
	if h == nil {
		h = slog.Default().Handler()
	}
	if h.Enabled(context.Background(), slog.LevelDebug) {
		args = append([]any{String("logId", id.String())}, args...)
	}
	l.logWithCallerPC(slog.LevelError, msg, args...)
}

// InfoLog logs an info message with optional attributes (logId is omitted at INFO level).
func (l Logger) InfoLog(id LogId, msg string, args ...any) {
	l.Info(id, msg, args...)
}

// WarnLog logs a warning message with a LogId and optional attributes.
func (l Logger) WarnLog(id LogId, msg string, args ...any) {
	l.Warn(id, msg, args...)
}

// ErrorLog logs an error message with a LogId and optional attributes.
func (l Logger) ErrorLog(id LogId, msg string, args ...any) {
	l.Error(id, msg, args...)
}

var defaultLogger = NewLogger()

// Default() returns the default logger instance.
func Default() Logger {
	return defaultLogger
}

// SetDefault replaces the default logger instance.
func SetDefault(logger Logger) {
	defaultLogger = logger
}
