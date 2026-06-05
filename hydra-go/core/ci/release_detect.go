package ci

import (
	"fmt"

	"hydra-gitops.org/hydra/hydra-go/core/git"
)

// chartDirChangedSinceLastRelease reports whether files under relChartDir
// changed since the last build-* tag that included this directory, or since
// the repository root commit when no such tag exists.
func chartDirChangedSinceLastRelease(r *git.Repo, relChartDir string) (bool, error) {
	tag, err := r.LastBuildTag(relChartDir)
	if err == nil {
		return r.HasChanges(tag, "HEAD", relChartDir)
	}
	initHash, errInit := r.InitialCommitHash()
	if errInit != nil {
		return false, fmt.Errorf("change detection for %s: last build tag: %w; initial commit: %w", relChartDir, err, errInit)
	}
	return r.HasChanges(initHash, "HEAD", relChartDir)
}

func envAllowed(env string, allowed []string) bool {
	for _, e := range allowed {
		if e == env {
			return true
		}
	}
	return false
}
