package types

// InCluster is the ClusterName for the in-cluster configuration
const InCluster ClusterName = "in-cluster"

// Directory name constants used throughout the hydra package
const (
	// InClusterDir is the directory name for in-cluster configurations
	InClusterDir = string(InCluster)

	// ArgocdDir is the directory name for ArgoCD configurations
	ArgocdDir = "argocd"

	// RootDir is the expected directory name for root applications
	RootDir = "root"

	// HydraContext is the environment variable name containing the Hydra context path
	HydraContextEnvName = "HYDRA_CONTEXT"

	// HydraNoCacheEnvName disables persistent Helm template caching and related in-process caches when set to a truthy value.
	HydraNoCacheEnvName = "HYDRA_NO_CACHE"
)
