package directive

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSleepLineAndParse(t *testing.T) {
	line := SleepLine(1.2)
	assert.Equal(t, "#!hydra sleep 1.2\r\n", line)

	secs, ok := ParseSleepLine(line)
	require.True(t, ok)
	assert.InDelta(t, 1.2, secs, 1e-9)
}

func TestParseSleepLine_RejectsUnknownDirective(t *testing.T) {
	_, ok := ParseSleepLine("#!hydra wait 1\r\n")
	assert.False(t, ok)
}

func TestWriteSleep(t *testing.T) {
	var buf bytes.Buffer
	require.NoError(t, WriteSleep(&buf, 2))
	assert.Equal(t, "#!hydra sleep 2\r\n", buf.String())
}
