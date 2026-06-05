package ci

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNextChildChartWrapperVersion_SameDepValuesOnly(t *testing.T) {
	v, err := NextChildChartWrapperVersion("1.200.9", "dev", "1.200.9-dev")
	require.NoError(t, err)
	assert.Equal(t, "1.200.9-1-dev", v)
}

func TestNextChildChartWrapperVersion_ExtraChain(t *testing.T) {
	v, err := NextChildChartWrapperVersion("1.200.9", "dev", "1.200.9-1-dev")
	require.NoError(t, err)
	assert.Equal(t, "1.200.9-2-dev", v)
}

func TestNextChildChartWrapperVersion_DepBump(t *testing.T) {
	v, err := NextChildChartWrapperVersion("1.200.9", "dev", "1.200.8-dev")
	require.NoError(t, err)
	assert.Equal(t, "1.200.9-dev", v)
}

func TestNextChildChartWrapperVersion_Stage(t *testing.T) {
	v, err := NextChildChartWrapperVersion("2.0.0", "stage", "2.0.0-stage")
	require.NoError(t, err)
	assert.Equal(t, "2.0.0-1-stage", v)
}
