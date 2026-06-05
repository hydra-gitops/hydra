package cmd

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"hydra-gitops.org/hydra/hydra-go/cli/exitcode"
)

// shouldRunLess reports whether argv (os.Args[1:]) enables pager mode.
// Parsing stops at "--" so positional "--less" is not treated as a flag.
func shouldRunLess(args []string) bool {
	for _, a := range args {
		if a == "--" {
			return false
		}
		if a == "--less" {
			return true
		}
		if strings.HasPrefix(a, "--less=") {
			v := strings.TrimPrefix(a, "--less=")
			return v != "false" && v != "0"
		}
	}
	return false
}

func stripLessFlag(args []string) []string {
	out := make([]string, 0, len(args))
	for i := 0; i < len(args); i++ {
		a := args[i]
		if a == "--" {
			out = append(out, args[i:]...)
			break
		}
		if a == "--less" {
			continue
		}
		if strings.HasPrefix(a, "--less=") {
			continue
		}
		out = append(out, a)
	}
	return out
}

func injectColorForLessPipe(args []string) []string {
	if len(args) == 0 {
		return args
	}
	out := append([]string(nil), args...)
	var prepend []string
	if shouldInjectColor(out) {
		prepend = append(prepend, "--color")
	}
	if shouldInjectColorLog(out) {
		prepend = append(prepend, "--color-log")
	}
	if len(prepend) == 0 {
		return out
	}
	return append(prepend, out...)
}

func shouldInjectColor(args []string) bool {
	if hasExplicitColorOff(args) {
		return false
	}
	if hasColorForcedOn(args) {
		return false
	}
	return true
}

func shouldInjectColorLog(args []string) bool {
	if hasNoColorLogFlag(args) {
		return false
	}
	if hasJsonLogFlag(args) {
		return false
	}
	if hasColorLogExplicitOn(args) {
		return false
	}
	if hasColorLogExplicitOff(args) {
		return false
	}
	return true
}

func scanArgs(args []string, fn func(a string, i int) bool) bool {
	for i := 0; i < len(args); i++ {
		a := args[i]
		if a == "--" {
			return false
		}
		if fn(a, i) {
			return true
		}
	}
	return false
}

func hasExplicitColorOff(args []string) bool {
	return scanArgs(args, func(a string, i int) bool {
		if a == "--no-color" {
			return true
		}
		if a == "--color=false" || a == "-c=false" {
			return true
		}
		if strings.HasPrefix(a, "--color=") {
			v := strings.TrimPrefix(a, "--color=")
			if v == "false" || v == "0" {
				return true
			}
		}
		if strings.HasPrefix(a, "-c=") {
			v := strings.TrimPrefix(a, "-c=")
			if v == "false" || v == "0" {
				return true
			}
		}
		if a == "--color-mode=never" {
			return true
		}
		if a == "--color-mode" && i+1 < len(args) && args[i+1] == "never" {
			return true
		}
		return false
	})
}

func hasColorForcedOn(args []string) bool {
	return scanArgs(args, func(a string, i int) bool {
		if a == "--color" || a == "-c" || a == "--color=true" || a == "-c=true" {
			return true
		}
		if strings.HasPrefix(a, "--color=") {
			v := strings.TrimPrefix(a, "--color=")
			if v == "true" || v == "1" {
				return true
			}
		}
		if strings.HasPrefix(a, "-c=") {
			v := strings.TrimPrefix(a, "-c=")
			if v == "true" || v == "1" {
				return true
			}
		}
		if a == "--color-mode=always" {
			return true
		}
		if a == "--color-mode" && i+1 < len(args) && args[i+1] == "always" {
			return true
		}
		return false
	})
}

func hasNoColorLogFlag(args []string) bool {
	return scanArgs(args, func(a string, _ int) bool {
		return a == "--no-color-log"
	})
}

func hasJsonLogFlag(args []string) bool {
	return scanArgs(args, func(a string, _ int) bool {
		return a == "--json-log"
	})
}

func hasColorLogExplicitOn(args []string) bool {
	return scanArgs(args, func(a string, _ int) bool {
		if a == "--color-log" || a == "--color-log=true" {
			return true
		}
		if strings.HasPrefix(a, "--color-log=") {
			v := strings.TrimPrefix(a, "--color-log=")
			return v == "true" || v == "1"
		}
		return false
	})
}

func hasColorLogExplicitOff(args []string) bool {
	return scanArgs(args, func(a string, _ int) bool {
		if a == "--color-log=false" {
			return true
		}
		if strings.HasPrefix(a, "--color-log=") {
			v := strings.TrimPrefix(a, "--color-log=")
			return v == "false" || v == "0"
		}
		return false
	})
}

func pagerFromEnv() *exec.Cmd {
	p := strings.TrimSpace(os.Getenv("PAGER"))
	if p == "" {
		// -S: chop long lines; -R: ANSI color escapes; +G: open scrolled to end
		return exec.Command("less", "-SR", "+G")
	}
	return exec.Command("/bin/sh", "-c", p)
}

// runLessPipe spawns hydra without --less, merges stdout+stderr into one stream,
// and runs the pager in the foreground with the default process environment.
func runLessPipe(args []string) error {
	childArgs := injectColorForLessPipe(stripLessFlag(args))
	exe := os.Args[0]

	r, w, err := os.Pipe()
	if err != nil {
		return err
	}
	defer r.Close()

	cmd := exec.Command(exe, childArgs...)
	cmd.Stdout = w
	cmd.Stderr = w
	cmd.Stdin = os.Stdin
	cmd.Env = os.Environ()

	if err := cmd.Start(); err != nil {
		_ = w.Close()
		return err
	}
	if err := w.Close(); err != nil {
		return err
	}

	pager := pagerFromEnv()
	pager.Stdin = r
	pager.Stdout = os.Stdout
	pager.Stderr = os.Stderr
	pager.Env = os.Environ()

	errCh := make(chan error, 1)
	go func() { errCh <- cmd.Wait() }()

	pagerErr := pager.Run()
	if pagerErr != nil {
		_ = cmd.Process.Kill()
		<-errCh
		var ee *exec.ExitError
		if errors.As(pagerErr, &ee) {
			return exitcode.New(ee.ExitCode(), "")
		}
		return fmt.Errorf("pager: %w", pagerErr)
	}

	waitErr := <-errCh

	if waitErr != nil {
		var ee *exec.ExitError
		if errors.As(waitErr, &ee) {
			code := ee.ExitCode()
			if code != 0 {
				return exitcode.New(code, "")
			}
		}
		return waitErr
	}
	return nil
}
