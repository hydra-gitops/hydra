package flags

import (
	"hydra-gitops.org/hydra/hydra-go/core/types"
)

// Flags is a marker interface for all flag structs.
type Flags interface {
	Flags() Flags
}

// NewConfigFromFlags creates a Config from any flags struct that implements
// the relevant flag interfaces (WithColorFlag, WithKubernetesConnectionAllowedFlag, etc.)
// The argument must be a pointer (enforced by Flags interface using pointer receiver).
func NewConfigFromFlags(f Flags, kubernetesConnectionAllowed types.KubernetesConnectionAllowed) types.Config {
	// Set default values
	color := types.ColorNo
	dryRun := types.DryRunNo

	// Extract Color if available
	if colorFlag, ok := f.(WithColorFlag); ok {
		if flag := colorFlag.WithColorFlag(); flag != nil {
			color = flag.Color
		}
	}

	// Extract DryRun if available
	if dryRunFlag, ok := f.(WithDryRunFlag); ok {
		if flag := dryRunFlag.WithDryRunFlag(); flag != nil {
			dryRun = flag.DryRun
		}
	}

	// Extract KubernetesConnectionAllowed if available
	if k8sFlag, ok := f.(WithKubernetesConnectionAllowedFlag); ok {
		if flag := k8sFlag.WithKubernetesConnectionAllowedFlag(); flag != nil {
			kubernetesConnectionAllowed = flag.KubernetesConnectionAllowed
		}
	}

	// --no-cluster overrides to disallow cluster connections
	if noClusterFlag, ok := f.(WithNoClusterFlag); ok {
		if flag := noClusterFlag.WithNoClusterFlag(); flag != nil && flag.NoCluster {
			kubernetesConnectionAllowed = types.KubernetesConnectionAllowedNo
		}
	}

	helmTemplateCacheEnabled := !types.HelmTemplateCacheDisabledByEnv()
	if nc, ok := f.(WithNoCacheFlag); ok {
		if fl := nc.WithNoCacheFlag(); fl != nil && fl.NoCache {
			helmTemplateCacheEnabled = false
		}
	}

	return types.NewConfig(color, dryRun, kubernetesConnectionAllowed, helmTemplateCacheEnabled)
}
