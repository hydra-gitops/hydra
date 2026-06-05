package log

import (
	"context"
	"log/slog"
	"regexp"
)

// ReplacedMessage contains the result of placeholder replacement
type ReplacedMessage struct {
	Message  string          // The formatted message with replaced placeholders
	Replaced map[string]bool // Map of keys that were replaced
}

// ReplacePlaceholders replaces {key} placeholders in msg with values from attrMap.
// Supports: {key} or '{key}' (quotes must be on both sides or not at all)
// If messageColor and messageValueColor are provided, values will be colorized.
// Returns a ReplacedMessage with the formatted message and keys that were replaced.
func ReplacePlaceholders(
	msg string,
	attrMap map[string]string,
	messageColor string,
	messageValueColor string,
) ReplacedMessage {
	colorizeValues := messageColor != "" && messageValueColor != ""
	usedAttrsMap := make(map[string]bool)

	// Match either {key} or '{key}' but not mixed quotes
	re := regexp.MustCompile(`\{([^}]+)\}|'(\{[^}]+\})'`)
	result := re.ReplaceAllStringFunc(msg, func(m string) string {
		// Extract the key from the matched pattern
		var key string
		if m[0] == '{' {
			// Format: {key}
			keyMatch := regexp.MustCompile(`\{([^}]+)\}`).FindStringSubmatch(m)
			if len(keyMatch) < 2 {
				return m
			}
			key = keyMatch[1]
		} else {
			// Format: '{key}'
			keyMatch := regexp.MustCompile(`'\{([^}]+)\}'`).FindStringSubmatch(m)
			if len(keyMatch) < 2 {
				return m
			}
			key = keyMatch[1]
		}

		if val, ok := attrMap[key]; ok {
			usedAttrsMap[key] = true
			if colorizeValues {
				if m[0] == '\'' {
					// Format: '{key}' -> {{messageValueColor}}'value'{{messageColor}}
					return messageValueColor + "'" + val + "'" + messageColor
				}
				// Format: {key} -> {{messageValueColor}}value{{messageColor}}
				return messageValueColor + val + messageColor
			}
			if m[0] == '\'' {
				return "'" + val + "'"
			}
			return val
		}
		return m
	})

	return ReplacedMessage{
		Message:  result,
		Replaced: usedAttrsMap,
	}
}

// ReplacePlaceholdersWithArgs replaces {key} placeholders in msg with values from slog args.
// Supports: {key} or '{key}' (quotes must be on both sides or not at all)
// If messageColor and messageValueColor are provided, values will be colorized.
// Returns a ReplacedMessage with the formatted message and keys that were replaced.
func ReplacePlaceholdersWithArgs(
	msg string,
	args []any,
	messageColor string,
	messageValueColor string,
) ReplacedMessage {
	// Build attribute map for placeholder replacement
	attrMap := make(map[string]string)
	for i := range args {
		if keyAttr, ok := args[i].(slog.Attr); ok {
			attrMap[keyAttr.Key] = keyAttr.Value.String()
		}
	}

	// Use ReplacePlaceholders with the built attribute map
	return ReplacePlaceholders(msg, attrMap, messageColor, messageValueColor)
}

// FormatOptions contains options for the formatHandler
type FormatOptions struct {
	RemoveUsedAttrs   bool   // If true, remove attributes that were used in placeholders
	AddTemplate       bool   // If true, add the unformatted message as a "template" attribute
	MessageColor      string // ANSI color code for message text
	MessageValueColor string // ANSI color code for values in message
}

// ColorizeValues returns true if values should be colorized (colors must be set)
func (opts FormatOptions) ColorizeValues() bool {
	return opts.MessageColor != "" && opts.MessageValueColor != ""
}

// formatHandler replaces {key} placeholders in the message with attribute values
type formatHandler struct {
	handler slog.Handler
	opts    FormatOptions
}

// NewFormatHandler creates a new format handler
func NewFormatHandler(handler slog.Handler, opts FormatOptions) slog.Handler {
	return &formatHandler{
		handler: handler,
		opts:    opts,
	}
}

func (h *formatHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.handler.Enabled(ctx, level)
}

func (h *formatHandler) Handle(ctx context.Context, r slog.Record) error {
	// Build attribute map for placeholder replacement
	attrMap := make(map[string]string)

	r.Attrs(func(a slog.Attr) bool {
		attrMap[a.Key] = a.Value.String()
		return true
	})

	// Store original message for template if AddTemplate is enabled
	originalMessage := r.Message

	// Replace placeholders in message with colorization
	replaced := ReplacePlaceholders(r.Message, attrMap, h.opts.MessageColor, h.opts.MessageValueColor)
	r.Message = replaced.Message

	// Add template attribute if configured
	if h.opts.AddTemplate {
		r.AddAttrs(slog.String("template", originalMessage))
	}

	// Remove used attributes if configured
	if h.opts.RemoveUsedAttrs && len(replaced.Replaced) > 0 {
		attrs := []slog.Attr{}
		r.Attrs(func(a slog.Attr) bool {
			if !replaced.Replaced[a.Key] {
				attrs = append(attrs, a)
			}
			return true
		})

		// Create new record without used attributes
		newRecord := slog.NewRecord(r.Time, r.Level, r.Message, r.PC)
		for _, a := range attrs {
			newRecord.AddAttrs(a)
		}
		r = newRecord
	}

	return h.handler.Handle(ctx, r)
}

func (h *formatHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &formatHandler{
		handler: h.handler.WithAttrs(attrs),
		opts:    h.opts,
	}
}

func (h *formatHandler) WithGroup(name string) slog.Handler {
	return &formatHandler{
		handler: h.handler.WithGroup(name),
		opts:    h.opts,
	}
}
