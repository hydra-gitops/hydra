package references

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestEventRefParserYAML_DocumentsGenericRelatedAndRegardingEdges guards the embedded Event parser
// that exposes generic outgoing refs for related/regarding ownership fallback.
func TestEventRefParserYAML_DocumentsGenericRelatedAndRegardingEdges(t *testing.T) {
	data, err := refParsersFS.ReadFile("ref-parsers/kubernetes/events.k8s.io_v1/Event.yaml")
	require.NoError(t, err)
	s := string(data)
	assert.Contains(t, s, "entity.related")
	assert.Contains(t, s, "label('related')")
	assert.Contains(t, s, "entity.regarding")
	assert.Contains(t, s, ".refType('regarding')")
}

func TestWorkloadRegardingEventParserYAML_DocumentsTemplateWorkloadFallback(t *testing.T) {
	data, err := refParsersFS.ReadFile("ref-parsers/kubernetes/workload_regarding_events.yaml")
	require.NoError(t, err)
	s := string(data)
	assert.Contains(t, s, "apps/v1/Deployment")
	assert.Contains(t, s, "label('workloadRegardingEvent')")
	assert.Contains(t, s, "e.entity.regarding.kind == 'Pod'")
	assert.Contains(t, s, "e.entity.involvedObject.kind == 'Pod'")
}
