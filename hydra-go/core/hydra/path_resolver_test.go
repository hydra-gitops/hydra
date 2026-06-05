package hydra

import (
	"path/filepath"
	"testing"

	"hydra-gitops.org/hydra/hydra-go/base/log"
	"hydra-gitops.org/hydra/hydra-go/core/types"
)

func resetCaches() {
	contextCache = nil
	clusterCache = nil
	rootAppCache = nil
	childAppCache = nil
}

func TestResolvePath(t *testing.T) {
	// Get the testdata directory
	testdataDir, err := filepath.Abs(filepath.Join(".", "..", "..", "testdata"))
	if err != nil {
		t.Fatalf("Failed to get testdata directory: %v", err)
	}

	tests := []struct {
		name            string
		path            string
		expectedType    string
		expectedCluster types.ClusterName
		expectedRootApp types.RootAppName
		shouldError     bool
	}{
		{
			name:         "Test directory - Context",
			path:         filepath.Join(testdataDir, "test"),
			expectedType: "context",
			shouldError:  false,
		},
		{
			name:            "Test/in-cluster - Cluster",
			path:            filepath.Join(testdataDir, "test", "in-cluster"),
			expectedType:    "cluster",
			expectedCluster: "in-cluster",
			shouldError:     false,
		},
		{
			name:            "Test/in-cluster/cluster-infra - RootApp",
			path:            filepath.Join(testdataDir, "test", "in-cluster", "cluster-infra"),
			expectedType:    "rootapp",
			expectedCluster: "in-cluster",
			expectedRootApp: "cluster-infra",
			shouldError:     false,
		},
		{
			name:            "Test/target - Cluster",
			path:            filepath.Join(testdataDir, "test", "target"),
			expectedType:    "cluster",
			expectedCluster: "target",
			shouldError:     false,
		},
		{
			name:            "Test/target/cluster-infra - RootApp",
			path:            filepath.Join(testdataDir, "test", "target", "cluster-infra"),
			expectedType:    "rootapp",
			expectedCluster: "target",
			expectedRootApp: "cluster-infra",
			shouldError:     false,
		},
		{
			name:            "Test/target/example - RootApp",
			path:            filepath.Join(testdataDir, "test", "target", "example"),
			expectedType:    "rootapp",
			expectedCluster: "target",
			expectedRootApp: "example",
			shouldError:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hydra, err := ResolvePath(log.Default(), types.HydraContext(tt.path), types.NewConfig(types.ColorNo, types.DryRunYes, types.KubernetesConnectionAllowedNo, true))

			if (err != nil) != tt.shouldError {
				t.Errorf("ResolvePath() error = %v, shouldError %v", err, tt.shouldError)
				return
			}

			if err != nil {
				return
			}

			switch tt.expectedType {
			case "context":
				ctx := hydra.AsContext()
				if ctx == nil {
					t.Errorf("Expected Context, but got nil")
					return
				}
				if hydra.AsCluster() != nil {
					t.Errorf("Expected Context, but AsCluster() returned non-nil")
				}
				if hydra.AsRootApp() != nil {
					t.Errorf("Expected Context, but AsRootApp() returned non-nil")
				}

			case "cluster":
				cluster := hydra.AsCluster()
				if cluster == nil {
					t.Errorf("Expected Cluster, but got nil")
					return
				}
				if cluster.ClusterName != tt.expectedCluster {
					t.Errorf("Expected ClusterName %q, but got %q", tt.expectedCluster, cluster.ClusterName)
				}
				if hydra.AsContext() == nil {
					t.Errorf("Expected AsContext() to return non-nil for Cluster")
				}
				if hydra.AsRootApp() != nil {
					t.Errorf("Expected Cluster, but AsRootApp() returned non-nil")
				}

			case "rootapp":
				rootApp := hydra.AsRootApp()
				if rootApp == nil {
					t.Errorf("Expected RootApp, but got nil")
					return
				}
				if rootApp.ClusterName != tt.expectedCluster {
					t.Errorf("Expected ClusterName %q, but got %q", tt.expectedCluster, rootApp.ClusterName)
				}
				if rootApp.RootAppName != tt.expectedRootApp {
					t.Errorf("Expected RootAppName %q, but got %q", tt.expectedRootApp, rootApp.RootAppName)
				}
				if hydra.AsContext() == nil {
					t.Errorf("Expected AsContext() to return non-nil for RootApp")
				}
				if hydra.AsCluster() == nil {
					t.Errorf("Expected AsCluster() to return non-nil for RootApp")
				}
			}
		})
	}
}

func TestContextCacheRespectsKubernetesConnectionAllowed(t *testing.T) {
	resetCaches()
	defer resetCaches()

	testdataDir, err := filepath.Abs(filepath.Join(".", "..", "..", "testdata"))
	if err != nil {
		t.Fatalf("Failed to get testdata directory: %v", err)
	}
	testPath := filepath.Join(testdataDir, "test")

	offlineConfig := types.NewConfig(types.ColorNo, types.DryRunNo, types.KubernetesConnectionAllowedNo, true)
	ctx1, err := ResolvePath(log.Default(), types.HydraContext(testPath), offlineConfig)
	if err != nil {
		t.Fatalf("ResolvePath with KubernetesConnectionAllowedNo failed: %v", err)
	}
	if ctx1.Config().KubernetesConnectionAllowed() != types.KubernetesConnectionAllowedNo {
		t.Fatal("Expected KubernetesConnectionAllowedNo for first context")
	}

	onlineConfig := types.NewConfig(types.ColorNo, types.DryRunNo, types.KubernetesConnectionAllowedYes, true)
	ctx2, err := ResolvePath(log.Default(), types.HydraContext(testPath), onlineConfig)
	if err != nil {
		t.Fatalf("ResolvePath with KubernetesConnectionAllowedYes failed: %v", err)
	}
	if ctx2.Config().KubernetesConnectionAllowed() != types.KubernetesConnectionAllowedYes {
		t.Fatalf("Expected KubernetesConnectionAllowedYes for second context, but got KubernetesConnectionAllowedNo (cache returned stale config)")
	}
}
