package commands

import (
	"fmt"

	"hydra-gitops.org/hydra/hydra-go/core/hydra"
	"k8s.io/client-go/discovery"
)

// KubernetesServerMinorVersion returns the Kubernetes minor from the live API server's version info.
// It fails when the minor field has no leading digits (for example custom builds that use "+"
// without a numeric minor).
func KubernetesServerMinorVersion(c *hydra.Cluster) (int, error) {
	if c == nil {
		return 0, fmt.Errorf("cluster is nil")
	}
	restConfig, err := RestConfigForHydra(c)
	if err != nil {
		return 0, err
	}
	dc, err := discovery.NewDiscoveryClientForConfig(restConfig)
	if err != nil {
		return 0, err
	}
	ver, err := dc.ServerVersion()
	if err != nil {
		return 0, err
	}
	m := parseLeadingInt(ver.Minor)
	if m <= 0 {
		m = ParseKubernetesMinorFromVersionString(ver.GitVersion)
	}
	if m <= 0 {
		return 0, fmt.Errorf("could not parse server minor from gitVersion=%q minorField=%q", ver.GitVersion, ver.Minor)
	}
	return m, nil
}
