package progress

import (
	"bytes"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestReadLines_splitsOnNewlineAndRest(t *testing.T) {
	var got [][]byte
	err := readLines(strings.NewReader("a\nb\nc"), func(line []byte) error {
		got = append(got, append([]byte(nil), line...))
		return nil
	})
	require.NoError(t, err)
	require.Equal(t, [][]byte{[]byte("a\n"), []byte("b\n"), []byte("c")}, got)
}

func TestReadLines_trailingWithoutNewline(t *testing.T) {
	var got [][]byte
	err := readLines(strings.NewReader("only"), func(line []byte) error {
		got = append(got, append([]byte(nil), line...))
		return nil
	})
	require.NoError(t, err)
	require.Equal(t, [][]byte{[]byte("only")}, got)
}

func TestReadLines_emitErrorStops(t *testing.T) {
	boom := errors.New("boom")
	err := readLines(strings.NewReader("a\nb"), func(line []byte) error {
		return boom
	})
	require.ErrorIs(t, err, boom)
}

func TestReadLines_emptyReader(t *testing.T) {
	n := 0
	err := readLines(bytes.NewReader(nil), func(line []byte) error {
		n++
		return nil
	})
	require.NoError(t, err)
	require.Equal(t, 0, n)
}

func TestReadLines_ioReaderPartial(t *testing.T) {
	r := io.MultiReader(strings.NewReader("x\n"), strings.NewReader("y"))
	var got [][]byte
	err := readLines(r, func(line []byte) error {
		got = append(got, append([]byte(nil), line...))
		return nil
	})
	require.NoError(t, err)
	require.Equal(t, [][]byte{[]byte("x\n"), []byte("y")}, got)
}
