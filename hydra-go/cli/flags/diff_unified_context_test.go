package flags

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestUnifiedDiffContextLines_Default(t *testing.T) {
	assert.Equal(t, 3, UnifiedDiffContextLines(-1, -1, -1))
}

func TestUnifiedDiffContextLines_CombinedMax(t *testing.T) {
	assert.Equal(t, 5, UnifiedDiffContextLines(2, 5, -1))
	assert.Equal(t, 4, UnifiedDiffContextLines(-1, -1, 4))
	assert.Equal(t, 0, UnifiedDiffContextLines(0, 0, -1))
}
