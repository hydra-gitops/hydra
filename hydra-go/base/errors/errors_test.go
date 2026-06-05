package errors

import (
	"errors"
	"testing"
)

// testError implements the Error interface for testing
type testError struct {
	id  ErrorId
	msg string
}

func (e *testError) Error() string {
	return e.msg
}

func (e *testError) ErrorId() ErrorId {
	return e.id
}

func TestId_WithKnownError(t *testing.T) {
	err := &testError{id: ErrInternalError, msg: "test error"}

	result := Id(err)

	if result != ErrInternalError {
		t.Errorf("expected %s, got %s", ErrInternalError, result)
	}
}

func TestId_WithUnknownError(t *testing.T) {
	err := errors.New("standard error")

	result := Id(err)

	if result != ErrUnknown {
		t.Errorf("expected %s, got %s", ErrUnknown, result)
	}
}

func TestId_WithNil(t *testing.T) {
	result := Id(nil)

	if result != ErrUnknown {
		t.Errorf("expected %s, got %s", ErrUnknown, result)
	}
}

func TestIsKnownError_WithKnownError(t *testing.T) {
	err := &testError{id: ErrHydraConfigError, msg: "config error"}

	if !IsKnownError(err) {
		t.Error("expected IsKnownError to return true for known error")
	}
}

func TestIsKnownError_WithUnknownError(t *testing.T) {
	err := errors.New("standard error")

	if IsKnownError(err) {
		t.Error("expected IsKnownError to return false for unknown error")
	}
}

func TestIsKnownError_WithNil(t *testing.T) {
	if IsKnownError(nil) {
		t.Error("expected IsKnownError to return false for nil")
	}
}

func TestErrorId_MatchesError_True(t *testing.T) {
	err := &testError{id: ErrKeyNotFound, msg: "key not found"}

	if !ErrKeyNotFound.MatchesError(err) {
		t.Error("expected MatchesError to return true for matching error")
	}
}

func TestErrorId_MatchesError_False(t *testing.T) {
	err := &testError{id: ErrKeyNotFound, msg: "key not found"}

	if ErrInternalError.MatchesError(err) {
		t.Error("expected MatchesError to return false for non-matching error")
	}
}

func TestErrorId_MatchesError_StandardError(t *testing.T) {
	err := errors.New("standard error")

	if ErrInternalError.MatchesError(err) {
		t.Error("expected MatchesError to return false for standard error")
	}
}

func TestErrorId_MatchesError_Nil(t *testing.T) {
	if ErrInternalError.MatchesError(nil) {
		t.Error("expected MatchesError to return false for nil")
	}
}

func TestErrorId_Constants(t *testing.T) {
	// Verify that error IDs are unique and non-empty
	errorIds := []ErrorId{
		ErrAborted,
		ErrAppIdIsNoRootApp,
		ErrAppIdsDifferentClusters,
		ErrAppNotEnabled,
		ErrCelCompileFailed,
		ErrHydraConfigError,
		ErrInternalError,
		ErrKeyNotFound,
		ErrKeyTypeMismatch,
	}

	seen := make(map[ErrorId]bool)
	for _, id := range errorIds {
		if id == "" {
			t.Errorf("error ID should not be empty")
		}
		if seen[id] {
			t.Errorf("duplicate error ID: %s", id)
		}
		seen[id] = true
	}
}

func TestErrUnknown_IsEmpty(t *testing.T) {
	if ErrUnknown != "" {
		t.Errorf("ErrUnknown should be empty string, got %q", ErrUnknown)
	}
}
