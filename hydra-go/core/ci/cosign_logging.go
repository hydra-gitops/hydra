package ci

import (
	"bufio"
	"io"
	stdlog "log"
	"os"
	"strings"
	"sync"

	"hydra-gitops.org/hydra/hydra-go/base/log"
)

var cosignLogRedirectMu sync.Mutex

func withCosignHydraLogger(fn func() error) error {
	cosignLogRedirectMu.Lock()
	defer cosignLogRedirectMu.Unlock()

	l := log.Default()

	stdoutReader, stdoutWriter, err := os.Pipe()
	if err != nil {
		return err
	}
	stderrReader, stderrWriter, err := os.Pipe()
	if err != nil {
		_ = stdoutReader.Close()
		_ = stdoutWriter.Close()
		return err
	}

	oldStdout := os.Stdout
	oldStderr := os.Stderr
	oldStdlogWriter := stdlog.Writer()
	oldStdlogFlags := stdlog.Flags()
	oldStdlogPrefix := stdlog.Prefix()

	os.Stdout = stdoutWriter
	os.Stderr = stderrWriter
	stdlog.SetOutput(stderrWriter)
	stdlog.SetFlags(0)
	stdlog.SetPrefix("")

	var wg sync.WaitGroup
	wg.Add(2)
	go scanCosignStream(&wg, stdoutReader, l, false)
	go scanCosignStream(&wg, stderrReader, l, true)

	runErr := fn()

	_ = stdoutWriter.Close()
	_ = stderrWriter.Close()
	os.Stdout = oldStdout
	os.Stderr = oldStderr
	stdlog.SetOutput(oldStdlogWriter)
	stdlog.SetFlags(oldStdlogFlags)
	stdlog.SetPrefix(oldStdlogPrefix)

	wg.Wait()

	return runErr
}

func scanCosignStream(wg *sync.WaitGroup, r *os.File, l log.Logger, isErr bool) {
	defer wg.Done()
	defer r.Close()

	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for scanner.Scan() {
		emitCosignLine(l, scanner.Text(), isErr)
	}
	if err := scanner.Err(); err != nil && err != io.EOF {
		l.DebugLog(logIdCI, "cosign log redirect error: {err}", log.Err(err))
	}
}

func emitCosignLine(l log.Logger, line string, isErr bool) {
	msg := strings.TrimSpace(line)
	if msg == "" {
		return
	}

	if strings.HasPrefix(msg, "WARNING: ") {
		l.DebugLog(logIdCI, "cosign: {message}", log.String("message", strings.TrimPrefix(msg, "WARNING: ")))
		return
	}

	_ = isErr
	l.DebugLog(logIdCI, "cosign: {message}", log.String("message", msg))
}
