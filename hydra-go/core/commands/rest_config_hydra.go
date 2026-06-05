package commands

import (
	"hydra-gitops.org/hydra/hydra-go/core/hydra"
	"hydra-gitops.org/hydra/hydra-go/core/k8s"
	"k8s.io/client-go/rest"
)

// RestConfigForHydra returns a REST config from the Hydra Kubernetes context.
// When h resolves to a [hydra.Cluster], [hydra.Cluster.RESTClientLimits] are applied; otherwise
// client-go defaults are used.
func RestConfigForHydra(h hydra.Hydra) (*rest.Config, error) {
	configFlags, err := hydra.HydraClusterAccess(h)
	if err != nil {
		return nil, err
	}
	restConfig, err := configFlags.ToRESTConfig()
	if err != nil {
		return nil, err
	}
	var lim hydra.RESTClientLimits
	if c := h.AsCluster(); c != nil {
		lim = c.RESTClientLimits
	}
	k8s.ApplyRESTConfigRateLimits(restConfig, lim.QPS, lim.Burst)
	return restConfig, nil
}
