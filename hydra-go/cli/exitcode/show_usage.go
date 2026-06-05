package exitcode

import (
	"errors"
)

// ShowUsageError wraps an error so the Hydra CLI prints Cobra command usage when the command fails.
// All unwrapped errors suppress usage by default; use WithShowUsage only when the user should see flags and examples.
type ShowUsageError struct {
	Err error
}

func (e *ShowUsageError) Error() string {
	if e == nil || e.Err == nil {
		return ""
	}
	return e.Err.Error()
}

func (e *ShowUsageError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}

// WithShowUsage wraps err so the CLI prints command usage on failure. Nil returns nil.
func WithShowUsage(err error) error {
	if err == nil {
		return nil
	}
	return &ShowUsageError{Err: err}
}

// IsShowUsage reports whether err or any error in its chain is a *ShowUsageError.
func IsShowUsage(err error) bool {
	var e *ShowUsageError
	return errors.As(err, &e)
}
