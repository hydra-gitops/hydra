package ci

import (
	"fmt"
	"path/filepath"

	"hydra-gitops.org/hydra/hydra-go/core/git"
)

// ReleaseChildEntry describes one child chart updated by the release pipeline.
type ReleaseChildEntry struct {
	Group, App, Env, Path  string
	OldVersion, NewVersion string
}

// ReleaseResult summarizes a release run.
type ReleaseResult struct {
	Children []ReleaseChildEntry
}

// RunRelease detects changed child charts, bumps wrapper and root versions,
// and runs mode-specific actions (dry-run log, local commit+tags, or CI stub).
func RunRelease(configPath string, mode Mode, targetBranch string) (ReleaseResult, error) {
	dir := filepath.Dir(configPath)
	cfg, err := LoadConfig(dir)
	if err != nil {
		return ReleaseResult{}, fmt.Errorf("load config: %w", err)
	}

	absChartsPath, err := filepath.Abs(filepath.Join(dir, cfg.CI.RootAppsPath))
	if err != nil {
		return ReleaseResult{}, fmt.Errorf("resolve charts path: %w", err)
	}

	repo := git.Open(absChartsPath)
	if repo.Err != nil {
		return ReleaseResult{}, fmt.Errorf("open repo: %w", repo.Err)
	}

	relChartsPath, err := filepath.Rel(repo.Path(), absChartsPath)
	if err != nil {
		return ReleaseResult{}, fmt.Errorf("relativize charts path: %w", err)
	}
	cfg.CI.RootAppsPath = filepath.ToSlash(relChartsPath)

	pattern := filepath.Join(cfg.CI.RootAppsPath, "*", "*", "*")
	matches, err := filepath.Glob(filepath.Join(repo.Path(), pattern))
	if err != nil {
		return ReleaseResult{}, fmt.Errorf("glob charts: %w", err)
	}

	var plan []ReleaseChildEntry
	for _, absPath := range matches {
		relPath, errRel := filepath.Rel(repo.Path(), absPath)
		if errRel != nil {
			continue
		}
		relPath = filepath.ToSlash(relPath)
		group, app, env, errP := ParseChartPath(relPath)
		if errP != nil {
			continue
		}
		if !envAllowed(env, cfg.CI.Environments) {
			continue
		}
		if app == "root" {
			continue
		}

		changed, errCh := chartDirChangedSinceLastRelease(repo, relPath)
		if errCh != nil {
			return ReleaseResult{}, errCh
		}
		if !changed {
			continue
		}

		ch, errL := repo.LoadChart(relPath)
		if errL != nil {
			return ReleaseResult{}, fmt.Errorf("load chart %s: %w", relPath, errL)
		}
		oldVer := ch.GetVersion()
		dep, errDep := dependencyVersionForWrapperRelease(ch, oldVer, relPath)
		if errDep != nil {
			return ReleaseResult{}, errDep
		}
		newVer, errN := NextChildChartWrapperVersion(dep, env, oldVer)
		if errN != nil {
			return ReleaseResult{}, fmt.Errorf("chart %s: %w", relPath, errN)
		}
		plan = append(plan, ReleaseChildEntry{
			Group:      group,
			App:        app,
			Env:        env,
			Path:       relPath,
			OldVersion: oldVer,
			NewVersion: newVer,
		})
	}

	if len(plan) == 0 {
		return ReleaseResult{}, nil
	}

	roots, errR := buildRootChartUpdates(repo, cfg, plan)
	if errR != nil {
		return ReleaseResult{}, errR
	}

	exec := NewReleaseExecutor(mode, targetBranch)
	return exec.run(repo, cfg, plan, roots)
}

// dependencyVersionForWrapperRelease selects the semver used to derive the next
// child wrapper version. When Chart.yaml lists dependencies, the version of
// the matching (or first) dependency is used. Otherwise the semver base is
// taken from the chart's own version field (major.minor.patch plus any Helm
// pre-release segment, without Hydra's environment suffix or extra counter) so
// standalone charts still get correct -dev / -stage / prod suffixes from
// NextChildChartWrapperVersion.
func dependencyVersionForWrapperRelease(ch *git.Chart, oldVer, relPath string) (string, error) {
	if v := ch.PrimaryDependencyVersion(); v != "" {
		return v, nil
	}
	parsed, err := ParseChartVersion(oldVer)
	if err != nil {
		return "", fmt.Errorf("chart %s: Chart.yaml has no dependencies; chart version must be a Hydra wrapper semver (e.g. 1.2.3-dev): %w", relPath, err)
	}
	return parsed.BaseVersion(), nil
}
