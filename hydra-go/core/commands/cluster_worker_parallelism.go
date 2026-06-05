package commands

import "runtime"

// EffectiveClusterWorkerParallelism resolves --parallel for cluster discovery listing, uninstall
// ref filtering, entity merge (when callers pass the CLI value), and apply SSA dry-run: a positive
// value is the worker count (capped at 64); zero means [runtime.GOMAXPROCS](0), clamped to [1, 64].
// Negative values are treated as 1 (defensive; CLI layers should reject negatives).
func EffectiveClusterWorkerParallelism(requested int) int {
	if requested < 0 {
		return 1
	}
	if requested == 0 {
		p := runtime.GOMAXPROCS(0)
		if p < 1 {
			return 1
		}
		if p > 64 {
			return 64
		}
		return p
	}
	if requested > 64 {
		return 64
	}
	return requested
}
