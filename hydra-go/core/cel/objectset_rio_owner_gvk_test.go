package cel

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestObjectsetRioOwnerGVKToHydraGvk(t *testing.T) {
	t.Parallel()
	cases := []struct {
		in   string
		want string
	}{
		{`/v1, Kind=Service`, `v1/Service`},
		{` /v1, Kind=Service `, `v1/Service`},
		{`v1, Kind=Service`, `v1/Service`},
		{`apps/v1, Kind=Deployment`, `apps/v1/Deployment`},
		{`networking.k8s.io/v1, Kind=Ingress`, `networking.k8s.io/v1/Ingress`},
		{`batch/v1, Kind=Job`, `batch/v1/Job`},
		{`, KIND=Foo`, ``},
		{`apps/v1, kind=Deployment`, `apps/v1/Deployment`},
		{``, ``},
		{`no-kind-separator`, ``},
		{`too/many/slashes/v1, Kind=X`, ``},
	}
	for i, tc := range cases {
		t.Run(fmt.Sprintf("%02d", i), func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tc.want, ObjectsetRioOwnerGVKToHydraGvk(tc.in))
		})
	}
}
