package hydra

import (
	"testing"

	"hydra-gitops.org/hydra/hydra-go/core/types"
	"github.com/stretchr/testify/assert"
)

func TestApiVersionNormalizationDedupKey(t *testing.T) {
	k := ApiVersionNormalizationDedupKey(
		"kafka.strimzi.io/v1beta2/KafkaTopic",
		types.Version("v1"),
	)
	assert.Equal(t,
		"kafka.strimzi.io/v1beta2/KafkaTopic\tv1",
		k)
}

func TestRememberLoggedApiVersionNormalization_inMemoryOnly(t *testing.T) {
	c := &Cluster{}
	key := ApiVersionNormalizationDedupKey("apps/v1/Deployment", types.Version("v1"))

	assert.False(t, c.HasLoggedApiVersionNormalization(key))
	c.RememberLoggedApiVersionNormalization(key)
	assert.True(t, c.HasLoggedApiVersionNormalization(key))
}
