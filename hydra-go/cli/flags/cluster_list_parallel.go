package flags

// ClusterListParallelFlag configures the shared --parallel worker count for cluster listing
// (VisitResources), cluster review ref-ownership passes, cluster uninstall filter/merge work,
// and hydra local/cluster review when talking to the API (see defineClusterListParallelFlag usage).
// Zero means GOMAXPROCS (capped at 64); footer shows one status line per worker when the
// effective count is greater than one.
type ClusterListParallelFlag struct {
	Parallel int
}

// WithClusterListParallelFlag is implemented by commands that support --parallel for ListClusterAll-style work.
type WithClusterListParallelFlag interface {
	WithClusterListParallelFlag() *ClusterListParallelFlag
}
