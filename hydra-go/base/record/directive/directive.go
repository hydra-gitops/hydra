package directive

import (
	"fmt"
	"io"
	"strconv"
	"strings"
)

const prefix = "#!hydra "

// SleepLine returns a terminal comment that sets the timestamp for the next cast event.
// The line uses CRLF so recordings match Hydra help output.
func SleepLine(seconds float64) string {
	return fmt.Sprintf("%ssleep %s\r\n", prefix, formatSeconds(seconds))
}

// WriteSleep writes a sleep directive to w (for terminal output during recordings).
func WriteSleep(w io.Writer, seconds float64) error {
	_, err := io.WriteString(w, SleepLine(seconds))
	return err
}

// ParseSleepLine reports whether line is a "#!hydra sleep <seconds>" directive.
func ParseSleepLine(line string) (seconds float64, ok bool) {
	content := strings.TrimRight(line, "\r\n")
	content = strings.TrimSpace(content)
	if !strings.HasPrefix(content, prefix) {
		return 0, false
	}
	rest := strings.TrimSpace(strings.TrimPrefix(content, prefix))
	if !strings.HasPrefix(rest, "sleep ") {
		return 0, false
	}
	secStr := strings.TrimSpace(strings.TrimPrefix(rest, "sleep "))
	secs, err := strconv.ParseFloat(secStr, 64)
	if err != nil || secs < 0 {
		return 0, false
	}
	return secs, true
}

func formatSeconds(seconds float64) string {
	return strconv.FormatFloat(seconds, 'f', -1, 64)
}
