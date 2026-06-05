package hydra

import (
	"hydra-gitops.org/hydra/hydra-go/core/types"
)

// HydraApp is an interface that extends Hydra and represents an application (either RootApp or ChildApp).
// It does not define additional methods beyond what Hydra already provides.
type HydraApp interface {
	Hydra

	AppId() types.AppId

	Namespace(types.HelmNetworkMode) (types.Namespace, error)

	Template(types.HelmNetworkMode, types.KubernetesVersionOrFallback) (types.YamlString, error)
}
