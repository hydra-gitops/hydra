package commands

import (
	"testing"

	"hydra-gitops.org/hydra/hydra-go/core/types"
	"github.com/stretchr/testify/assert"
)

func TestPickerRowStatusLocalTemplateOnly(t *testing.T) {
	id := types.Id("v1/ConfigMap/ns/cm")
	m := PickerRowStatusLocalTemplateOnly([]types.Id{id})
	assert.Equal(t, "missing", m[id])
}
