package commands

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParseKubernetesMinorFromVersionString(t *testing.T) {
	t.Parallel()
	assert.Equal(t, 30, ParseKubernetesMinorFromVersionString("1.30"))
	assert.Equal(t, 30, ParseKubernetesMinorFromVersionString("v1.30.0"))
	assert.Equal(t, 28, ParseKubernetesMinorFromVersionString("1.28.15-gke.100"))
	assert.Equal(t, 0, ParseKubernetesMinorFromVersionString(""))
	assert.Equal(t, 0, ParseKubernetesMinorFromVersionString("garbage"))
}

func TestEffectiveMinorForLocalBootstrapCatalog(t *testing.T) {
	t.Parallel()
	assert.Equal(t, 99, effectiveMinorForLocalBootstrapCatalog(0))
	assert.Equal(t, 30, effectiveMinorForLocalBootstrapCatalog(30))
}
