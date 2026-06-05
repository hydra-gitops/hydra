package types

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDiffMode_Parse_Server(t *testing.T) {
	dm, err := DiffModeEnumType.Parse("server")
	require.NoError(t, err)
	assert.Equal(t, DiffModeServer, dm)
}

func TestDiffMode_Parse_Raw(t *testing.T) {
	dm, err := DiffModeEnumType.Parse("raw")
	require.NoError(t, err)
	assert.Equal(t, DiffModeRaw, dm)
}

func TestDiffMode_Parse_Invalid(t *testing.T) {
	_, err := DiffModeEnumType.Parse("invalid")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid diff mode")
}

func TestDiffMode_String_Server(t *testing.T) {
	assert.Equal(t, "server", DiffModeServer.String())
}

func TestDiffMode_String_Raw(t *testing.T) {
	assert.Equal(t, "raw", DiffModeRaw.String())
}

func TestDiffMode_Stringify(t *testing.T) {
	s, err := DiffModeEnumType.Stringify(DiffModeServer)
	require.NoError(t, err)
	assert.Equal(t, "server", s)

	s, err = DiffModeEnumType.Stringify(DiffModeRaw)
	require.NoError(t, err)
	assert.Equal(t, "raw", s)
}

func TestDiffMode_Values(t *testing.T) {
	values := DiffModeEnumType.Values()
	assert.Equal(t, []DiffMode{DiffModeServer, DiffModeRaw}, values)
}

func TestDiffMode_Valid(t *testing.T) {
	assert.True(t, DiffModeEnumType.Valid(DiffModeServer))
	assert.True(t, DiffModeEnumType.Valid(DiffModeRaw))
	assert.False(t, DiffModeEnumType.Valid(DiffMode(99)))
}

func TestDiffMode_ServerIsDefault(t *testing.T) {
	values := DiffModeEnumType.Values()
	assert.Equal(t, DiffModeServer, values[0], "server should be the first (default) value")
}
