package yaml

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// TestToYamlFromYaml_preservesLeadingNewlineInStringValue is a minimal round-trip check:
// a plain map value "\nhello" must survive ToYaml → FromYaml unchanged.
//
// Background: gopkg.in/yaml.v3 (go-yaml) can drop a leading newline on marshal/unmarshal round-trips
// (Node.Encode internally re-parses emitted YAML; literal-block encoding is involved). Upstream
// discussions include https://github.com/go-yaml/yaml/issues/968 and https://github.com/go-yaml/yaml/issues/972
// (and related links there). Hydra works around this in ToYaml via preserveLeadingNewlines; this test
// guards that contract if the library or workaround changes.
func TestToYamlFromYaml_preservesLeadingNewlineInStringValue(t *testing.T) {
	in := map[string]any{"v": "\nhello"}
	ys, err := ToYaml(in)
	require.NoError(t, err)
	out, err := FromYaml[map[string]any](ys)
	require.NoError(t, err)
	require.Equal(t, "\nhello", out["v"].(string))
}
