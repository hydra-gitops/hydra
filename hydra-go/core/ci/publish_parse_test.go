package ci

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestChartRelPathFromReleaseTag_DocumentationTable(t *testing.T) {
	root := "apps"
	groups := sortGroupsByNameLengthDesc([]string{"demo", "cluster-infra"})
	tests := []struct {
		tag    string
		want   string
		wantOK bool
	}{
		{"build-202603051555", "", false},
		{"demo-service-ui-1.200.9-dev", "apps/demo/service-ui/dev", true},
		{"demo-service-ui-1.200.9-stage", "apps/demo/service-ui/stage", true},
		{"demo-service-ui-1.200.9", "apps/demo/service-ui/prod", true},
		{"demo-service-backend-1.5.1-2b866935-dev", "apps/demo/service-backend/dev", true},
		{"demo-service-backend-1.5.1-2b866935-stage", "apps/demo/service-backend/stage", true},
		{"demo-service-backend-1.5.1-2b866935", "apps/demo/service-backend/prod", true},
		{"demo-root-1.200.9-dev", "apps/demo/root/dev", true},
		{"cluster-infra-ingress-nginx-4.11.0-dev", "apps/cluster-infra/ingress-nginx/dev", true},
		{"cluster-infra-ingress-nginx-4.11.0", "apps/cluster-infra/ingress-nginx/prod", true},
	}
	for _, tt := range tests {
		t.Run(tt.tag, func(t *testing.T) {
			got, ok, err := chartRelPathFromReleaseTag(tt.tag, root, groups)
			require.NoError(t, err)
			assert.Equal(t, tt.wantOK, ok)
			if tt.wantOK {
				assert.Equal(t, tt.want, got)
			}
		})
	}
}

func TestSortGroupsByNameLengthDesc(t *testing.T) {
	in := []string{"demo", "cluster-infra", "demo-infra"}
	got := sortGroupsByNameLengthDesc(in)
	assert.Equal(t, []string{"cluster-infra", "demo-infra", "demo"}, got)
}
