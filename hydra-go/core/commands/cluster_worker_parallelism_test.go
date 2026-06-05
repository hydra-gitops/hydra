package commands

import (
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestEffectiveClusterWorkerParallelism(t *testing.T) {
	t.Parallel()
	assert.GreaterOrEqual(t, EffectiveClusterWorkerParallelism(0), 1)
	assert.LessOrEqual(t, EffectiveClusterWorkerParallelism(0), 64)
	assert.Equal(t, 3, EffectiveClusterWorkerParallelism(3))
	assert.Equal(t, 1, EffectiveClusterWorkerParallelism(1))
	assert.Equal(t, 64, EffectiveClusterWorkerParallelism(99))
	assert.Equal(t, 1, EffectiveClusterWorkerParallelism(-3))
	n := runtime.GOMAXPROCS(0)
	got := EffectiveClusterWorkerParallelism(0)
	switch {
	case n < 1:
		assert.Equal(t, 1, got)
	case n > 64:
		assert.Equal(t, 64, got)
	default:
		assert.Equal(t, n, got)
	}
}
