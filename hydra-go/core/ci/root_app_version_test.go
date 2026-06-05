package ci

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBumpRootAppChartVersion_Dev(t *testing.T) {
	v, err := BumpRootAppChartVersion("200.22.0-dev")
	require.NoError(t, err)
	assert.Equal(t, "200.23.0-dev", v)
}

func TestBumpRootAppChartVersion_Prod(t *testing.T) {
	v, err := BumpRootAppChartVersion("200.22.0")
	require.NoError(t, err)
	assert.Equal(t, "200.23.0", v)
}

func TestNextRootAppChartVersionAfterChildChanges_ExtraOnlyPatch(t *testing.T) {
	v, err := NextRootAppChartVersionAfterChildChanges("200.22.0-dev", []ReleaseChildEntry{{
		OldVersion: "1.0.0-dev",
		NewVersion: "1.0.0-1-dev",
	}})
	require.NoError(t, err)
	assert.Equal(t, "200.22.1-dev", v)
}

func TestNextRootAppChartVersionAfterChildChanges_BaseChangeMinor(t *testing.T) {
	v, err := NextRootAppChartVersionAfterChildChanges("200.22.0-dev", []ReleaseChildEntry{{
		OldVersion: "1.0.0-dev",
		NewVersion: "1.0.1-dev",
	}})
	require.NoError(t, err)
	assert.Equal(t, "200.23.0-dev", v)
}

func TestNextRootAppChartVersionAfterChildChanges_MultipleAllExtraOnly(t *testing.T) {
	v, err := NextRootAppChartVersionAfterChildChanges("5.1.0-dev", []ReleaseChildEntry{
		{OldVersion: "1.0.0-dev", NewVersion: "1.0.0-1-dev"},
		{OldVersion: "2.0.0-1-dev", NewVersion: "2.0.0-2-dev"},
	})
	require.NoError(t, err)
	assert.Equal(t, "5.1.1-dev", v)
}

func TestNextRootAppChartVersionAfterChildChanges_MultipleForcesMinor(t *testing.T) {
	v, err := NextRootAppChartVersionAfterChildChanges("5.1.0-dev", []ReleaseChildEntry{
		{OldVersion: "1.0.0-dev", NewVersion: "1.0.0-1-dev"},
		{OldVersion: "1.0.0-dev", NewVersion: "1.0.1-dev"},
	})
	require.NoError(t, err)
	assert.Equal(t, "5.2.0-dev", v)
}

func TestNextRootAppChartVersionAfterChildChanges_EmptyErr(t *testing.T) {
	_, err := NextRootAppChartVersionAfterChildChanges("1.0.0-dev", nil)
	require.Error(t, err)
}

func TestNormalizeChartVersionEnv(t *testing.T) {
	tests := []struct {
		name    string
		current string
		env     string
		want    string
	}{
		{name: "keeps dev suffix", current: "200.22.1-dev", env: "dev", want: "200.22.1-dev"},
		{name: "adds missing dev suffix", current: "200.22.1", env: "dev", want: "200.22.1-dev"},
		{name: "switches wrong suffix", current: "200.22.1-stage", env: "dev", want: "200.22.1-dev"},
		{name: "normalizes prod to no suffix", current: "200.22.1-dev", env: "prod", want: "200.22.1"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NormalizeChartVersionEnv(tt.current, tt.env)
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestSameChildWrapperBaseVersion(t *testing.T) {
	tests := []struct {
		name string
		old  string
		new  string
		same bool
	}{
		{"extra only dev", "1.0.0-dev", "1.0.0-1-dev", true},
		{"extra chain dev", "1.200.9-dev", "1.200.9-2-dev", true},
		{"patch dep bump in wrapper", "1.0.0-dev", "1.0.1-dev", false},
		{"prerelease plus extra", "1.5.1-2b866935-dev", "1.5.1-2b866935-1-dev", true},
		{"minor dep bump in wrapper", "1.0.0-dev", "1.1.0-dev", false},
		{"extra stage", "1.0.0-stage", "1.0.0-1-stage", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := SameChildWrapperBaseVersion(tt.old, tt.new)
			require.NoError(t, err)
			assert.Equal(t, tt.same, got)
		})
	}
}
