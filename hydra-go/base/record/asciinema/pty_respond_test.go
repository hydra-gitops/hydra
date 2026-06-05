package asciinema

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRespondPTYQueriesInBuffer_WritesDummyReplies(t *testing.T) {
	respR, ptmx, err := os.Pipe()
	require.NoError(t, err)
	defer respR.Close()
	defer ptmx.Close()

	in := []byte("prompt\x1b]11;?\x1b\\\x1b[6nhelp\r\n")
	out := respondPTYQueriesInBuffer(ptmx, in)
	assert.Equal(t, "prompthelp\r\n", string(out))

	require.NoError(t, ptmx.Close())
	resp, err := io.ReadAll(respR)
	require.NoError(t, err)
	combined := string(resp)
	assert.True(t, strings.Contains(combined, "R") || strings.Contains(combined, "rgb:"))
}

func TestSplitPTYQueryCarry_HoldsIncompletePrefix(t *testing.T) {
	stable, carry := splitPTYQueryCarry([]byte("ok\x1b[6"))
	assert.Equal(t, "ok", string(stable))
	assert.Equal(t, "\x1b[6", string(carry))
}

func TestCaptureScriptOutput_HydraHelpUnderOneSecond(t *testing.T) {
	hydraBin := os.Getenv("HYDRA_RECORD_TEST_BIN")
	if hydraBin == "" {
		t.Skip("set HYDRA_RECORD_TEST_BIN to a built hydra binary for timing test")
	}
	script := filepath.Join(t.TempDir(), "rec.sh")
	require.NoError(t, os.WriteFile(script, []byte("#!/bin/bash\nset -euo pipefail\nexport TERM=xterm-256color\n"+
		hydraBin+" gitops apply --help\n"), 0o755))

	t0 := time.Now()
	_, err := captureScriptOutput(script, recordingEnv(), nil)
	require.NoError(t, err)
	assert.Less(t, time.Since(t0), time.Second)
}
