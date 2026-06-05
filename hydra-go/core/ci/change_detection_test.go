package ci

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPromoteBranch(t *testing.T) {
	tests := []struct {
		name      string
		targetEnv string
		group     string
		app       string
		expected  string
	}{
		{
			name:      "dev to stage",
			targetEnv: "stage",
			group:     "demo",
			app:       "service-ui",
			expected:  "hydra/promote/to-stage/demo/service-ui",
		},
		{
			name:      "stage to prod",
			targetEnv: "prod",
			group:     "demo",
			app:       "service-ui",
			expected:  "hydra/promote/to-prod/demo/service-ui",
		},
		{
			name:      "cluster infra app",
			targetEnv: "stage",
			group:     "cluster-infra",
			app:       "ingress-nginx",
			expected:  "hydra/promote/to-stage/cluster-infra/ingress-nginx",
		},
		{
			name:      "demo-infra app",
			targetEnv: "prod",
			group:     "demo-infra",
			app:       "postgres",
			expected:  "hydra/promote/to-prod/demo-infra/postgres",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := PromoteBranch(tt.targetEnv, tt.group, tt.app)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestAppTag(t *testing.T) {
	tests := []struct {
		name     string
		group    string
		app      string
		version  string
		expected string
	}{
		{
			name:     "demo service dev",
			group:    "demo",
			app:      "service-ui",
			version:  "1.200.9-dev",
			expected: "demo-service-ui-1.200.9-dev",
		},
		{
			name:     "demo service prod",
			group:    "demo",
			app:      "service-ui",
			version:  "1.200.9",
			expected: "demo-service-ui-1.200.9",
		},
		{
			name:     "cluster-infra app",
			group:    "cluster-infra",
			app:      "ingress-nginx",
			version:  "4.11.0-dev",
			expected: "cluster-infra-ingress-nginx-4.11.0-dev",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := AppTag(tt.group, tt.app, tt.version)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestRootAppTag(t *testing.T) {
	tests := []struct {
		name     string
		group    string
		version  string
		expected string
	}{
		{
			name:     "demo root dev",
			group:    "demo",
			version:  "200.23.0-dev",
			expected: "demo-root-200.23.0-dev",
		},
		{
			name:     "cluster-infra root prod",
			group:    "cluster-infra",
			version:  "41.0.0",
			expected: "cluster-infra-root-41.0.0",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := RootAppTag(tt.group, tt.version)
			assert.Equal(t, tt.expected, result)
		})
	}
}
