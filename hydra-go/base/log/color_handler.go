package log

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"runtime"
	"sync"

	"hydra-gitops.org/hydra/hydra-go/base/colors"
)

// colorHandler implements a colored slog.Handler
type colorHandler struct {
	mu         *sync.Mutex
	w          io.Writer
	opts       slog.HandlerOptions
	preAttrs   []slog.Attr
	groups     []string
	timestamps bool
	colors     *ColorHandlerColors // Color codes, nil if noColor is set
}

// ColorHandlerColors contains color codes for the handler
type ColorHandlerColors struct {
	LevelDebug     colors.Color
	LevelInfo      colors.Color
	LevelWarn      colors.Color
	LevelError     colors.Color
	Message        colors.Color
	MessageValue   colors.Color
	Attribute      colors.Color
	AttributeKey   colors.Color
	AttributeValue colors.Color
	Source         colors.Color
}

// ColorHandlerOptions contains options for NewColorHandler
type ColorHandlerOptions struct {
	HandlerOpts *slog.HandlerOptions
	Colors      *ColorHandlerColors
	Timestamps  bool
	// Output is the destination for log records. If nil, [os.Stderr] is used.
	Output io.Writer
}

// DefaultColors returns the default color scheme
func DefaultColors() *ColorHandlerColors {
	return &ColorHandlerColors{
		LevelDebug:     colors.LightCyan,
		LevelInfo:      colors.LightGreen,
		LevelWarn:      colors.LightYellow,
		LevelError:     colors.LightRed,
		Message:        colors.LightWhite,
		MessageValue:   colors.LightMagenta,
		Attribute:      colors.White,
		AttributeKey:   colors.Cyan,
		AttributeValue: colors.Green,
		Source:         colors.LightGray,
	}
}

// NewColorHandler creates a new colored slog handler.
// If no writer is provided in the options, it defaults to os.Stderr.
func NewColorHandler(opts *ColorHandlerOptions) slog.Handler {
	if opts == nil {
		opts = &ColorHandlerOptions{}
	}

	handlerOpts := opts.HandlerOpts
	if handlerOpts == nil {
		handlerOpts = &slog.HandlerOptions{}
	}

	w := opts.Output
	if w == nil {
		w = os.Stderr
	}

	return &colorHandler{
		mu:         &sync.Mutex{},
		w:          w,
		opts:       *handlerOpts,
		colors:     opts.Colors,
		timestamps: opts.Timestamps,
	}
}

func (h *colorHandler) Enabled(ctx context.Context, level slog.Level) bool {
	minLevel := slog.LevelInfo
	if h.opts.Level != nil {
		minLevel = h.opts.Level.Level()
	}
	return level >= minLevel
}

func (h *colorHandler) Handle(ctx context.Context, r slog.Record) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	var buf []byte

	// Time
	if h.timestamps && !r.Time.IsZero() {
		buf = append(buf, r.Time.Format("15:04:05.000")...)
		buf = append(buf, ' ')
	}

	// Level (with color)
	if h.colors != nil {
		switch r.Level {
		case slog.LevelDebug:
			buf = append(buf, h.colors.LevelDebug.String()...)
			buf = append(buf, "DEBUG"...)
			buf = append(buf, colors.Reset.String()...)
		case slog.LevelInfo:
			buf = append(buf, h.colors.LevelInfo.String()...)
			buf = append(buf, "INFO "...)
			buf = append(buf, colors.Reset.String()...)
		case slog.LevelWarn:
			buf = append(buf, h.colors.LevelWarn.String()...)
			buf = append(buf, "WARN "...)
			buf = append(buf, colors.Reset.String()...)
		case slog.LevelError:
			buf = append(buf, h.colors.LevelError.String()...)
			buf = append(buf, "ERROR"...)
			buf = append(buf, colors.Reset.String()...)
		default:
			buf = append(buf, r.Level.String()...)
		}
	} else {
		// No colors
		switch r.Level {
		case slog.LevelDebug:
			buf = append(buf, "DEBUG"...)
		case slog.LevelInfo:
			buf = append(buf, "INFO "...)
		case slog.LevelWarn:
			buf = append(buf, "WARN "...)
		case slog.LevelError:
			buf = append(buf, "ERROR"...)
		default:
			buf = append(buf, r.Level.String()...)
		}
	}
	buf = append(buf, ' ')

	// Message
	msg := r.Message
	if h.colors != nil {
		buf = append(buf, h.colors.Message.String()...)
		buf = append(buf, msg...)
		buf = append(buf, colors.Reset.String()...)
	} else {
		// No colors
		buf = append(buf, msg...)
	}

	// Attributes (stack trace is appended separately after the source line)
	var stackTrace string
	r.Attrs(func(a slog.Attr) bool {
		if a.Key == "stack" {
			stackTrace = a.Value.String()
			return true
		}
		buf = h.appendAttr(buf, a)
		return true
	})

	// Source at the end (only if AddSource is true and no_source is not set in context)
	if h.opts.AddSource && !IsNoSource(ctx) {
		fs := runtime.CallersFrames([]uintptr{r.PC})
		f, _ := fs.Next()
		if f.File != "" {
			buf = append(buf, ' ')
			if h.colors != nil {
				buf = append(buf, h.colors.Source.String()...)
			}
			// Extract package/filename from full path
			dir, file := filepath.Split(f.File)
			pkg := filepath.Base(dir)
			buf = append(buf, '(')
			if f.Function != "" {
				buf = append(buf, f.Function...)
				buf = append(buf, '@')
			}
			buf = append(buf, fmt.Sprintf("%s/%s:%d)", pkg, file, f.Line)...)
			if h.colors != nil {
				buf = append(buf, colors.Reset.String()...)
			}
		}
	}

	buf = append(buf, '\n')

	// Append stack trace after the main log line (for error-level messages)
	if stackTrace != "" {
		if h.colors != nil {
			buf = append(buf, h.colors.Source.String()...)
		}
		buf = append(buf, stackTrace...)
		if h.colors != nil {
			buf = append(buf, colors.Reset.String()...)
		}
	}

	_, err := h.w.Write(buf)
	return err
}

func (h *colorHandler) appendAttr(buf []byte, a slog.Attr) []byte {
	buf = append(buf, ' ')
	if h.colors != nil {
		buf = append(buf, h.colors.AttributeKey.String()...)
		buf = append(buf, a.Key...)
		buf = append(buf, colors.Reset.String()...)
		buf = append(buf, '=')
		buf = append(buf, h.colors.AttributeValue.String()...)
		val := a.Value.String()
		buf = append(buf, val...)
		buf = append(buf, colors.Reset.String()...)
	} else {
		buf = append(buf, a.Key...)
		buf = append(buf, '=')
		val := a.Value.String()
		buf = append(buf, val...)
	}
	return buf
}

func (h *colorHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &colorHandler{
		mu:       h.mu,
		w:        h.w,
		opts:     h.opts,
		preAttrs: append(h.preAttrs, attrs...),
		groups:   h.groups,
		colors:   h.colors,
	}
}

func (h *colorHandler) WithGroup(name string) slog.Handler {
	return &colorHandler{
		mu:       h.mu,
		w:        h.w,
		opts:     h.opts,
		preAttrs: h.preAttrs,
		groups:   append(h.groups, name),
		colors:   h.colors,
	}
}
