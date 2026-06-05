package hydra

import (
	"hydra-gitops.org/hydra/hydra-go/base/log"
	"hydra-gitops.org/hydra/hydra-go/core/types"
)

// Hydra is an interface that can be implemented by Context, Cluster, RootApp, and ChildApp types.
// It provides methods to convert between these types and process app references.
type Hydra interface {
	// L returns the logger for this Hydra type.
	// Use with LogId: h.L().DebugLog(logIdContext, "message", ...)
	L() log.Logger

	// AsContext returns this value as a Context if it represents a Context, otherwise nil.
	// Returns nil if the type cannot be converted to Context.
	AsContext() *Context

	// AsCluster returns this value as a Cluster if it represents a Cluster, otherwise nil.
	// Returns nil if the type cannot be converted to Cluster.
	AsCluster() *Cluster

	// AsRootApp returns this value as a RootApp if it represents a RootApp, otherwise nil.
	// Returns nil if the type cannot be converted to RootApp.
	AsRootApp() *RootApp

	// AsChildApp returns this value as a ChildApp if it represents a ChildApp, otherwise nil.
	// Returns nil if the type cannot be converted to ChildApp.
	AsChildApp() *ChildApp

	// AsApp returns this value as a HydraApp if it represents either a RootApp or ChildApp, otherwise nil.
	AsApp() HydraApp

	// Config returns the configuration for this Hydra context.
	Config() types.Config

	// WithCluster appends the given cluster to the current Hydra context.
	// limits are stored on the returned Cluster and applied when building Kubernetes API clients.
	WithCluster(cluster types.ClusterName, limits RESTClientLimits) (*Cluster, error)

	// WithApp processes the Hydra value with the given app reference.
	// It validates the app reference against the current hydra type and returns
	// the appropriate Hydra implementation (Context, Cluster, RootApp, or ChildApp).
	WithApp(app types.AppId) (HydraApp, error)

	// LoadValues loads and merges global values for all levels.
	// The returned values depend on the hydra type:
	// - Context: loads group and context values
	// - Cluster: loads context and cluster values
	// - RootApp: loads context, cluster, and root app values
	// - ChildApp: loads all levels including child app specific values
	LoadValuesMap(mode types.HelmNetworkMode) (types.ValuesMap, error)

	// Description returns a human-readable description of the Hydra type.
	// For Context, it returns the context path.
	// For Cluster, it returns cluster information.
	// For RootApp and ChildApp, it returns application information.
	Description() string
}
