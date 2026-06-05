package log

import "log/slog"

// Debug logs a debug message with optional attributes.
func Debug(msg string, args ...any) {
	slog.Debug(msg, args...)
}

// InfoLog logs an info message with optional attributes.
func InfoLog(msg string, args ...any) {
	slog.Info(msg, args...)
}

// WarnLog logs a warning message with optional attributes.
func WarnLog(msg string, args ...any) {
	slog.Warn(msg, args...)
}

// ErrorLog logs an error message with optional attributes.
func ErrorLog(msg string, args ...any) {
	slog.Error(msg, args...)
}
