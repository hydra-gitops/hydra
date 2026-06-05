package references

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestPodMetricsRefParserYAML_DocumentsBidirectionalEdges guards the ref-parser file that powers
// merged inspect / uninstall propagation for PodMetrics (workloads/Pod ↔ PodMetrics).
func TestPodMetricsRefParserYAML_DocumentsBidirectionalEdges(t *testing.T) {
	data, err := refParsersFS.ReadFile("ref-parsers/metrics.k8s.io_v1beta1/PodMetrics.yaml")
	require.NoError(t, err)
	s := string(data)
	assert.Contains(t, s, "gvk: apps/v1/Deployment")
	assert.Contains(t, s, "gvk: v1/Pod")
	assert.Contains(t, s, "gvk: metrics.k8s.io/v1beta1/PodMetrics")
	assert.Contains(t, s, "- uninstall-safe")
	assert.Contains(t, s, `clusterEntities({"namespace": ns, "gvk": "metrics.k8s.io/v1beta1/PodMetrics"})`)
	assert.Contains(t, s, "metrics.k8s.io/v1beta1/PodMetrics")
	assert.Contains(t, s, "id('v1/Pod', ns, name)")
}
