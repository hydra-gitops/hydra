package commands

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMsgNonImmutableSSADryRunFailure_HintsReplaceWithoutFalseClaim(t *testing.T) {
	assert.Contains(t, msgNonImmutableSSADryRunFailure, "--replace")
	assert.False(t, strings.Contains(msgNonImmutableSSADryRunFailure, "--replace is set"),
		"message must not imply --replace was passed when it is only a hint")
}
