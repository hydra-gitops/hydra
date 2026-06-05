package flags

// ClusterListSkipOwnerRefsFlag configures hydra gitops list --skip-owner-refs.
type ClusterListSkipOwnerRefsFlag struct {
	SkipOwnerRefs bool
}

// WithClusterListSkipOwnerRefsFlag is implemented by commands that support --skip-owner-refs on cluster list.
type WithClusterListSkipOwnerRefsFlag interface {
	WithClusterListSkipOwnerRefsFlag() *ClusterListSkipOwnerRefsFlag
}
