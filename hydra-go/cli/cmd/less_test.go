package cmd

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestShouldRunLess(t *testing.T) {
	assert.False(t, shouldRunLess([]string{"gitops", "diff", "a.b"}))
	assert.True(t, shouldRunLess([]string{"gitops", "diff", "a.b", "--less"}))
	assert.True(t, shouldRunLess([]string{"--less", "version"}))
	assert.False(t, shouldRunLess([]string{"--less=false", "version"}))
	assert.False(t, shouldRunLess([]string{"--less=0", "version"}))
	assert.False(t, shouldRunLess([]string{"local", "template", "a.b", "--", "--less"}))
	assert.False(t, shouldRunLess([]string{"local", "source", "a.b", "--", "--less"}))
}

func TestStripLessFlag(t *testing.T) {
	assert.Equal(t,
		[]string{"gitops", "diff", "x.y"},
		stripLessFlag([]string{"gitops", "diff", "x.y", "--less"}))
	assert.Equal(t,
		[]string{"version"},
		stripLessFlag([]string{"--less", "version"}))
	assert.Equal(t,
		[]string{"local", "template", "a", "--", "--less"},
		stripLessFlag([]string{"--less", "local", "template", "a", "--", "--less"}))
	assert.Equal(t,
		[]string{"local", "source", "a", "--", "--less"},
		stripLessFlag([]string{"--less", "local", "source", "a", "--", "--less"}))
}

func TestInjectColorForLessPipe(t *testing.T) {
	assert.Equal(t,
		[]string{"--color", "--color-log", "version"},
		injectColorForLessPipe([]string{"version"}))

	assert.Equal(t,
		[]string{"--color-log", "version", "--no-color"},
		injectColorForLessPipe([]string{"version", "--no-color"}))

	assert.Equal(t,
		[]string{"--color", "version", "--json-log"},
		injectColorForLessPipe([]string{"version", "--json-log"}))

	assert.Equal(t,
		[]string{"--color", "version", "--color-log"},
		injectColorForLessPipe([]string{"version", "--color-log"}))

	assert.Equal(t,
		[]string{"--color", "version", "--no-color-log"},
		injectColorForLessPipe([]string{"version", "--no-color-log"}))

	assert.Equal(t,
		[]string{"--color-log", "version", "--color"},
		injectColorForLessPipe([]string{"version", "--color"}))
}
