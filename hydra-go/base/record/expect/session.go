package expect

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"time"

	"github.com/creack/pty"
	"hydra-gitops.org/hydra/hydra-go/base/colors"
)

// Session drives an interactive shell over a PTY, similar to Tcl expect.
type Session struct {
	cmd    *exec.Cmd
	pty    *os.File
	buffer bytes.Buffer
}

// StartBash starts /bin/bash as a login-free interactive shell with a PTY.
func StartBash() (*Session, error) {
	cmd := exec.Command("/bin/bash")
	cmd.Env = append(os.Environ(), "TERM=xterm-256color")
	ptyFile, err := pty.Start(cmd)
	if err != nil {
		return nil, fmt.Errorf("start bash with pty: %w", err)
	}
	return &Session{cmd: cmd, pty: ptyFile}, nil
}

// SetupGenericPrompt sets PS1/PS2 to the colored recording prompt.
func (s *Session) SetupGenericPrompt() error {
	return s.SendLine(`export PS1='` + colors.RecordingShellPS1() + `'; export PS2="$PS1"; unset PROMPT_COMMAND`)
}

// SendLine writes a line to the shell (including newline).
func (s *Session) SendLine(line string) error {
	if s.pty == nil {
		return errors.New("session is closed")
	}
	if _, err := io.WriteString(s.pty, line+"\n"); err != nil {
		return fmt.Errorf("write to pty: %w", err)
	}
	return nil
}

// Expect waits until substr appears in captured PTY output or timeout elapses.
func (s *Session) Expect(substr string, timeout time.Duration) error {
	if substr == "" {
		return errors.New("expect: empty substring")
	}
	deadline := time.Now().Add(timeout)
	buf := make([]byte, 4096)
	for time.Now().Before(deadline) {
		n, err := s.pty.Read(buf)
		if n > 0 {
			s.buffer.Write(buf[:n])
			if bytes.Contains(s.buffer.Bytes(), []byte(substr)) {
				return nil
			}
		}
		if err != nil {
			if errors.Is(err, os.ErrDeadlineExceeded) {
				time.Sleep(50 * time.Millisecond)
				continue
			}
			if err == io.EOF {
				break
			}
			return fmt.Errorf("read pty: %w", err)
		}
		time.Sleep(50 * time.Millisecond)
	}
	return fmt.Errorf("expect: substring %q not seen within %s", substr, timeout)
}

// Close shuts down the shell and PTY.
func (s *Session) Close() error {
	if s.pty != nil {
		_ = s.pty.Close()
		s.pty = nil
	}
	if s.cmd != nil && s.cmd.Process != nil {
		_ = s.cmd.Process.Kill()
		_ = s.cmd.Wait()
	}
	return nil
}
