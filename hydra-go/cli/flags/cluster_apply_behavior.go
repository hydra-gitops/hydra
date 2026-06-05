package flags

// ClusterApplyBehaviorFlags controls optional phases and bootstrap-related preparation for
// `hydra gitops apply`. Unless set (or implied by --bootstrap), those steps are skipped.
type ClusterApplyBehaviorFlags struct {
	SopsDecode      bool
	DownScaled      bool
	ScaleUp         bool
	OrphanScaleDown bool
	// SyncWindow is the raw --sync value; empty means the effective mode is resolved at runtime.
	SyncWindow      string
	BootstrapGuard  bool
	BootstrapClones bool
	BackupRestore   bool
	DisableWebhooks bool
	// Parallel is the number of concurrent SSA dry-run patch calls during apply planning (footer shows one status line per worker when >1). Zero means GOMAXPROCS (capped at 64 by the commands layer).
	Parallel int
}

type WithClusterApplyBehaviorFlags interface {
	WithClusterApplyBehaviorFlags() *ClusterApplyBehaviorFlags
}

func (f *ClusterApplyBehaviorFlags) WithClusterApplyBehaviorFlags() *ClusterApplyBehaviorFlags {
	return f
}
