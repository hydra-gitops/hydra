package asciinema

import (
	"strings"

	"hydra-gitops.org/hydra/hydra-go/base/record/directive"
)

// defaultLineDelaySeconds is applied to each output line unless a sleep directive overrides it.
const defaultLineDelaySeconds = 0.01

// HelpCastDocumentationCommand builds the header command field for help recordings.
func HelpCastDocumentationCommand(recordedHydraCommand string) string {
	return "hydra record help -- " + recordedHydraCommand
}

type castEvent struct {
	time float64
	kind string
	data string
}

func linesToEvents(lines []string, kind string) []castEvent {
	var out []castEvent
	var pendingTime *float64
	for _, line := range lines {
		if secs, ok := directive.ParseSleepLine(line); ok {
			pendingTime = &secs
			continue
		}
		if strings.TrimRight(line, "\r\n") == "" {
			continue
		}
		if isExitCodeOnlyLine(line) {
			continue
		}
		t := defaultLineDelaySeconds
		if pendingTime != nil {
			t = *pendingTime
			pendingTime = nil
		}
		out = append(out, castEvent{time: t, kind: kind, data: line})
	}
	return out
}

// splitTerminalLines splits on line boundaries and keeps original line endings (\r\n, \r, or \n).
func splitTerminalLines(data string) []string {
	if data == "" {
		return nil
	}
	var lines []string
	start := 0
	for start < len(data) {
		contentEnd, ending := findLineEndingAt(data, start)
		if ending == "" {
			lines = append(lines, ensureTrailingLineEnding(data[start:]))
			break
		}
		lines = append(lines, data[start:contentEnd]+ending)
		start = contentEnd + len(ending)
	}
	return lines
}

func findLineEndingAt(data string, start int) (contentEnd int, ending string) {
	for i := start; i < len(data); i++ {
		switch data[i] {
		case '\r':
			if i+1 < len(data) && data[i+1] == '\n' {
				return i, "\r\n"
			}
			return i, "\r"
		case '\n':
			return i, "\n"
		}
	}
	return len(data), ""
}

func detectPrimaryLineEnding(data string) string {
	if strings.Contains(data, "\r\n") {
		return "\r\n"
	}
	if strings.Contains(data, "\r") {
		return "\r"
	}
	return "\n"
}

func ensureTrailingLineEnding(data string) string {
	_, ending := findLineEndingAt(data, 0)
	if ending != "" {
		return data
	}
	return data + detectPrimaryLineEnding(data)
}
