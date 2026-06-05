// Package exitcode defines errors that carry a process exit status for the Hydra CLI.
package exitcode

import (
	"errors"
	"fmt"
)

// Error is an error that maps to a specific non-zero exit code. Cobra does not
// treat errors specially. By default commands use SilenceUsage (see cmd package);
// return WithShowUsage(err) when usage should be printed on failure.
type Error struct {
	Code int
	Msg  string
}

func (e *Error) Error() string {
	if e == nil {
		return ""
	}
	return e.Msg
}

// New returns an error that should exit the process with the given code.
// A code of 0 is coerced to 1 so failures never exit successfully by mistake.
func New(code int, msg string) error {
	if code == 0 {
		code = 1
	}
	return &Error{Code: code, Msg: msg}
}

// Newf is like New with formatted message.
func Newf(code int, format string, args ...interface{}) error {
	return New(code, fmt.Sprintf(format, args...))
}

// Is reports whether err or any error in its chain is an *Error.
func Is(err error) bool {
	var e *Error
	return errors.As(err, &e)
}

// As returns the exit code carried by err, or (0, false) if err is not an *Error.
func As(err error) (code int, ok bool) {
	var e *Error
	if errors.As(err, &e) {
		return e.Code, true
	}
	return 0, false
}
