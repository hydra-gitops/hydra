package asciinema

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHelpCastDocumentationCommand(t *testing.T) {
	assert.Equal(t, "hydra record help -- hydra argocd sync manual --help",
		HelpCastDocumentationCommand("hydra argocd sync manual --help"))
}

func TestLinesToEvents_SleepDirective(t *testing.T) {
	lines := []string{" $ \r\n", "#!hydra sleep 1.2\r\n", "hydra --help\r\n"}
	out := linesToEvents(lines, "o")
	require.Len(t, out, 2)
	assert.InDelta(t, defaultLineDelaySeconds, out[0].time, 1e-9)
	assert.Equal(t, " $ \r\n", out[0].data)
	assert.InDelta(t, 1.2, out[1].time, 1e-9)
	assert.Equal(t, "hydra --help\r\n", out[1].data)
}

func TestLinesToEvents_DropsExitCodeAndBlankLines(t *testing.T) {
	lines := []string{"\r\n", "0\n", "hydra --help\r\n"}
	out := linesToEvents(lines, "o")
	require.Len(t, out, 1)
	assert.InDelta(t, defaultLineDelaySeconds, out[0].time, 1e-9)
	assert.Equal(t, "hydra --help\r\n", out[0].data)
}

func readCastEvents(t *testing.T, path string) []castEvent {
	t.Helper()
	lines := splitLines(mustRead(t, path))
	var events []castEvent
	for _, line := range lines[1:] {
		if line == "" {
			continue
		}
		var raw []any
		require.NoError(t, json.Unmarshal([]byte(line), &raw))
		data, _ := raw[2].(string)
		events = append(events, castEvent{
			time: raw[0].(float64),
			kind: raw[1].(string),
			data: data,
		})
	}
	return events
}

func readCastHeader(t *testing.T, path string) map[string]any {
	t.Helper()
	data := mustRead(t, path)
	idx := 0
	for i, b := range data {
		if b == '\n' {
			idx = i
			break
		}
	}
	var header map[string]any
	require.NoError(t, json.Unmarshal(data[:idx], &header))
	return header
}

func mustRead(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	return data
}

func splitLines(data []byte) []string {
	var lines []string
	start := 0
	for i, b := range data {
		if b == '\n' {
			lines = append(lines, string(data[start:i]))
			start = i + 1
		}
	}
	if start < len(data) {
		lines = append(lines, string(data[start:]))
	}
	return lines
}
