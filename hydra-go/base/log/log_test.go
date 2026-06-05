package log

import (
	"context"
	"fmt"
	"log/slog"
	"testing"

	"hydra-gitops.org/hydra/hydra-go/base/errors"
)

// captureHandler records slog.Records for test assertions.
type captureHandler struct {
	records []slog.Record
	level   slog.Level
}

func (h *captureHandler) Enabled(_ context.Context, level slog.Level) bool {
	return level >= h.level
}

func (h *captureHandler) Handle(_ context.Context, r slog.Record) error {
	h.records = append(h.records, r)
	return nil
}

func (h *captureHandler) WithAttrs(_ []slog.Attr) slog.Handler { return h }
func (h *captureHandler) WithGroup(_ string) slog.Handler      { return h }

func (h *captureHandler) hasAttr(idx int, key string) bool {
	if idx >= len(h.records) {
		return false
	}
	found := false
	h.records[idx].Attrs(func(a slog.Attr) bool {
		if a.Key == key {
			found = true
			return false
		}
		return true
	})
	return found
}

func withCaptureLogger(level slog.Level, fn func(h *captureHandler)) {
	old := slog.Default()
	defer slog.SetDefault(old)

	h := &captureHandler{level: level}
	slog.SetDefault(slog.New(h))
	fn(h)
}

func TestCreateError_ReturnsErrorWithId(t *testing.T) {
	err := CreateError(errors.ErrInternalError, "test error message")

	if err == nil {
		t.Fatal("Error should not return nil")
	}

	// Check error message
	if err.Error() != "test error message" {
		t.Errorf("expected 'test error message', got %q", err.Error())
	}

	// Check error ID
	typedErr, ok := err.(errors.Error)
	if !ok {
		t.Fatal("returned error should implement errors.Error")
	}
	if typedErr.ErrorId() != errors.ErrInternalError {
		t.Errorf("expected %s, got %s", errors.ErrInternalError, typedErr.ErrorId())
	}
}

func TestCreateError_WithPlaceholders(t *testing.T) {
	err := CreateError(errors.ErrKeyNotFound, "key {key} not found in {location}",
		slog.String("key", "myKey"),
		slog.String("location", "config"))

	expected := "key myKey not found in config"
	if err.Error() != expected {
		t.Errorf("expected %q, got %q", expected, err.Error())
	}
}

func TestCreateError_WithQuotedPlaceholders(t *testing.T) {
	err := CreateError(errors.ErrKeyNotFound, "key '{key}' not found",
		slog.String("key", "myKey"))

	expected := "key 'myKey' not found"
	if err.Error() != expected {
		t.Errorf("expected %q, got %q", expected, err.Error())
	}
}

func TestCreateWarn_ReturnsErrorWithId(t *testing.T) {
	err := CreateWarn(errors.ErrNothingTodo, "nothing to do")

	if err == nil {
		t.Fatal("Warn should not return nil")
	}

	typedErr, ok := err.(errors.Error)
	if !ok {
		t.Fatal("returned error should implement errors.Error")
	}
	if typedErr.ErrorId() != errors.ErrNothingTodo {
		t.Errorf("expected %s, got %s", errors.ErrNothingTodo, typedErr.ErrorId())
	}
}

func TestCreateInfo_ReturnsErrorWithId(t *testing.T) {
	err := CreateInfo(errors.ErrAborted, "operation aborted")

	if err == nil {
		t.Fatal("Info should not return nil")
	}

	typedErr, ok := err.(errors.Error)
	if !ok {
		t.Fatal("returned error should implement errors.Error")
	}
	if typedErr.ErrorId() != errors.ErrAborted {
		t.Errorf("expected %s, got %s", errors.ErrAborted, typedErr.ErrorId())
	}
}

func TestLogLazy_WithLazyError(t *testing.T) {
	logged := false
	err := &lazyError{
		id:      errors.ErrInternalError,
		message: "test",
		Log: func() {
			logged = true
		},
	}

	result := LogLazy(err)

	if !result {
		t.Error("LogLazy should return true for lazyError")
	}
	if !logged {
		t.Error("LogLazy should call Log function")
	}
}

func TestLogLazy_WithRegularError(t *testing.T) {
	err := fmt.Errorf("regular error")

	result := LogLazy(err)

	if result {
		t.Error("LogLazy should return false for regular error")
	}
}

func TestLogLazy_WithNil(t *testing.T) {
	result := LogLazy(nil)

	if result {
		t.Error("LogLazy should return false for nil")
	}
}

func TestLogLazy_ReturnedErrorAlreadyEmitted_DoesNotLog(t *testing.T) {
	withCaptureLogger(slog.LevelError, func(h *captureHandler) {
		inner := fmt.Errorf("inner")
		wrapped := ReturnedErrorAlreadyEmitted(inner)
		if !LogLazy(wrapped) {
			t.Fatal("LogLazy should return true")
		}
		if len(h.records) != 0 {
			t.Fatalf("expected no slog records, got %d", len(h.records))
		}
	})
}

func TestReturnedErrorAlreadyEmitted_ErrorId(t *testing.T) {
	inner := CreateError(errors.ErrInternalError, "x")
	w := ReturnedErrorAlreadyEmitted(inner)
	if errors.Id(w) != errors.ErrInternalError {
		t.Fatalf("expected ErrInternalError, got %s", errors.Id(w))
	}
}

func TestReplacePlaceholders_Simple(t *testing.T) {
	attrMap := map[string]string{
		"key":   "myKey",
		"value": "myValue",
	}

	result := ReplacePlaceholders("key={key}, value={value}", attrMap, "", "")

	if result.Message != "key=myKey, value=myValue" {
		t.Errorf("expected 'key=myKey, value=myValue', got %q", result.Message)
	}
	if !result.Replaced["key"] {
		t.Error("key should be marked as replaced")
	}
	if !result.Replaced["value"] {
		t.Error("value should be marked as replaced")
	}
}

func TestReplacePlaceholders_QuotedPlaceholders(t *testing.T) {
	attrMap := map[string]string{
		"name": "test",
	}

	result := ReplacePlaceholders("name is '{name}'", attrMap, "", "")

	if result.Message != "name is 'test'" {
		t.Errorf("expected \"name is 'test'\", got %q", result.Message)
	}
}

func TestReplacePlaceholders_MissingKey(t *testing.T) {
	attrMap := map[string]string{
		"key": "value",
	}

	result := ReplacePlaceholders("{key} and {missing}", attrMap, "", "")

	if result.Message != "value and {missing}" {
		t.Errorf("expected 'value and {missing}', got %q", result.Message)
	}
	if result.Replaced["missing"] {
		t.Error("missing key should not be marked as replaced")
	}
}

func TestReplacePlaceholders_WithColors(t *testing.T) {
	attrMap := map[string]string{
		"key": "value",
	}

	result := ReplacePlaceholders("key={key}", attrMap, "\033[0m", "\033[32m")

	// Should contain color codes
	expected := "key=\033[32mvalue\033[0m"
	if result.Message != expected {
		t.Errorf("expected %q, got %q", expected, result.Message)
	}
}

func TestReplacePlaceholders_EmptyAttrMap(t *testing.T) {
	result := ReplacePlaceholders("no {replacements}", map[string]string{}, "", "")

	if result.Message != "no {replacements}" {
		t.Errorf("expected 'no {replacements}', got %q", result.Message)
	}
	if len(result.Replaced) != 0 {
		t.Error("no keys should be replaced")
	}
}

func TestReplacePlaceholdersWithArgs(t *testing.T) {
	args := []any{
		slog.String("name", "test"),
		slog.Int("count", 42),
	}

	result := ReplacePlaceholdersWithArgs("name={name}, count={count}", args, "", "")

	if result.Message != "name=test, count=42" {
		t.Errorf("expected 'name=test, count=42', got %q", result.Message)
	}
}

func TestLogId_OmittedWithoutDebug_ErrorGo(t *testing.T) {
	withCaptureLogger(slog.LevelInfo, func(h *captureHandler) {
		err := CreateError(errors.ErrInternalError, "test error")
		LogLazy(err)

		if len(h.records) != 1 {
			t.Fatalf("expected 1 record, got %d", len(h.records))
		}
		if h.hasAttr(0, "logId") {
			t.Error("logId should NOT be present at Error level without debug")
		}
	})
}

func TestLogId_PresentWithDebug_ErrorGo(t *testing.T) {
	withCaptureLogger(slog.LevelDebug, func(h *captureHandler) {
		err := CreateError(errors.ErrInternalError, "test error")
		LogLazy(err)

		if len(h.records) != 1 {
			t.Fatalf("expected 1 record, got %d", len(h.records))
		}
		if !h.hasAttr(0, "logId") {
			t.Error("logId should be present at Error level with debug")
		}
	})
}

func TestLogId_OmittedWithoutDebug_WarnGo(t *testing.T) {
	withCaptureLogger(slog.LevelInfo, func(h *captureHandler) {
		err := CreateWarn(errors.ErrNothingTodo, "test warn")
		LogLazy(err)

		if len(h.records) != 1 {
			t.Fatalf("expected 1 record, got %d", len(h.records))
		}
		if h.hasAttr(0, "logId") {
			t.Error("logId should NOT be present at Warn level without debug")
		}
	})
}

func TestLogId_OmittedWithoutDebug_LoggerWarnLog(t *testing.T) {
	withCaptureLogger(slog.LevelInfo, func(h *captureHandler) {
		l := NewLogger()
		l.Warn(Hydra().Child("test"), "test warn")

		if len(h.records) != 1 {
			t.Fatalf("expected 1 record, got %d", len(h.records))
		}
		if h.hasAttr(0, "logId") {
			t.Error("logId should NOT be present at WarnLog without debug")
		}
	})
}

func TestLogId_PresentWithDebug_LoggerWarnLog(t *testing.T) {
	withCaptureLogger(slog.LevelDebug, func(h *captureHandler) {
		l := NewLogger()
		l.Warn(Hydra().Child("test"), "test warn")

		if len(h.records) != 1 {
			t.Fatalf("expected 1 record, got %d", len(h.records))
		}
		if !h.hasAttr(0, "logId") {
			t.Error("logId should be present at WarnLog with debug")
		}
	})
}

func TestLogId_OmittedWithoutDebug_LoggerErrorLog(t *testing.T) {
	withCaptureLogger(slog.LevelInfo, func(h *captureHandler) {
		l := NewLogger()
		l.Error(Hydra().Child("test"), "test error")

		if len(h.records) != 1 {
			t.Fatalf("expected 1 record, got %d", len(h.records))
		}
		if h.hasAttr(0, "logId") {
			t.Error("logId should NOT be present at ErrorLog without debug")
		}
	})
}

func TestLogId_PresentWithDebug_LoggerErrorLog(t *testing.T) {
	withCaptureLogger(slog.LevelDebug, func(h *captureHandler) {
		l := NewLogger()
		l.Error(Hydra().Child("test"), "test error")

		if len(h.records) != 1 {
			t.Fatalf("expected 1 record, got %d", len(h.records))
		}
		if !h.hasAttr(0, "logId") {
			t.Error("logId should be present at ErrorLog with debug")
		}
	})
}

func TestAddSource_OnlyWithDebug(t *testing.T) {
	debugConfig := Config{Level: slog.LevelDebug}
	infoConfig := Config{Level: slog.LevelInfo}

	debugOpts := &slog.HandlerOptions{
		AddSource: debugConfig.Level <= slog.LevelDebug,
	}
	infoOpts := &slog.HandlerOptions{
		AddSource: infoConfig.Level <= slog.LevelDebug,
	}

	if !debugOpts.AddSource {
		t.Error("AddSource should be true when level is Debug")
	}
	if infoOpts.AddSource {
		t.Error("AddSource should be false when level is Info")
	}
}

func TestFormatOptions_ColorizeValues(t *testing.T) {
	tests := []struct {
		name     string
		opts     FormatOptions
		expected bool
	}{
		{
			name:     "both colors set",
			opts:     FormatOptions{MessageColor: "\033[0m", MessageValueColor: "\033[32m"},
			expected: true,
		},
		{
			name:     "no colors set",
			opts:     FormatOptions{},
			expected: false,
		},
		{
			name:     "only message color",
			opts:     FormatOptions{MessageColor: "\033[0m"},
			expected: false,
		},
		{
			name:     "only value color",
			opts:     FormatOptions{MessageValueColor: "\033[32m"},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.opts.ColorizeValues() != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, tt.opts.ColorizeValues())
			}
		})
	}
}
