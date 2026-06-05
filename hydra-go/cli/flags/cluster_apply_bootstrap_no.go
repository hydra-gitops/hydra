package flags

// ClusterApplyBootstrapNoFlags holds opt-out flags for the bundle implied by --bootstrap.
// Each field is true when the user passed the corresponding --no-* flag as true.
type ClusterApplyBootstrapNoFlags struct {
	NoSopsDecode      bool
	NoDownScaled      bool
	NoScaleUp         bool
	NoOrphanScaleDown bool
	NoBootstrapGuard  bool
	NoBootstrapClones bool
	NoBackupRestore   bool
	NoDisableWebhooks bool
}

// WithClusterApplyBootstrapNoFlags is implemented by command flag structs that support bootstrap opt-outs.
type WithClusterApplyBootstrapNoFlags interface {
	WithClusterApplyBootstrapNoFlags() *ClusterApplyBootstrapNoFlags
}

func (f *ClusterApplyBootstrapNoFlags) WithClusterApplyBootstrapNoFlags() *ClusterApplyBootstrapNoFlags {
	return f
}
