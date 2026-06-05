package exitcode

import (
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNew_CoercesZeroCode(t *testing.T) {
	err := New(0, "x")
	code, ok := As(err)
	require.True(t, ok)
	require.Equal(t, 1, code)
	require.Equal(t, "x", err.Error())
}

func TestNewf(t *testing.T) {
	err := Newf(2, "%d items", 3)
	code, ok := As(err)
	require.True(t, ok)
	require.Equal(t, 2, code)
	require.Equal(t, "3 items", err.Error())
}

func TestIs_Wrapped(t *testing.T) {
	inner := New(5, "inner")
	wrapped := fmt.Errorf("wrap: %w", inner)
	require.True(t, Is(wrapped))
	code, ok := As(wrapped)
	require.True(t, ok)
	require.Equal(t, 5, code)
}

func TestAs_NotExitCode(t *testing.T) {
	_, ok := As(errors.New("plain"))
	require.False(t, ok)
}

func TestWithShowUsage_Nil(t *testing.T) {
	require.Nil(t, WithShowUsage(nil))
}

func TestWithShowUsage_IsShowUsage(t *testing.T) {
	inner := errors.New("x")
	wrapped := WithShowUsage(inner)
	require.True(t, IsShowUsage(wrapped))
	require.Equal(t, "x", wrapped.Error())
	require.False(t, IsShowUsage(inner))
}

func TestWithShowUsage_UnwrapsForExitCode(t *testing.T) {
	inner := New(3, "fail")
	wrapped := WithShowUsage(inner)
	code, ok := As(wrapped)
	require.True(t, ok)
	require.Equal(t, 3, code)
	require.True(t, Is(wrapped))
}
