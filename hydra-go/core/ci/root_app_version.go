package ci

import "fmt"

// BumpRootAppChartVersion increments the middle component of the root app
// chart version (sprint.id.patch-env, e.g. 200.22.0-dev → 200.23.0-dev).
// Prefer NextRootAppChartVersionAfterChildChanges for release, which also
// increments the third component when every child change is extra-counter-only.
func BumpRootAppChartVersion(current string) (string, error) {
	v, err := ParseChartVersion(current)
	if err != nil {
		return "", err
	}
	v.Minor++
	return v.String(), nil
}

// NextRootAppChartVersionAfterChildChanges computes the new root app chart
// version after updating child pin versions. If every child has the same
// base as before (only the extra counter or env-wrapped re-release, e.g.
// 1.0.0-dev → 1.0.0-1-dev), the third version component is incremented
// (200.22.0-dev → 200.22.1-dev). If any child’s base semver or Helm
// pre-release segment changes (for example 1.0.0-dev → 1.0.1-dev), the middle
// component is incremented (200.22.0-dev → 200.23.0-dev) as in the previous
// behavior. Multiple children in one group use the stricter rule: a single
// base change forces a middle (minor) bump.
func NextRootAppChartVersionAfterChildChanges(current string, childEntries []ReleaseChildEntry) (string, error) {
	if len(childEntries) == 0 {
		return "", fmt.Errorf("no child version changes for root bump")
	}
	allExtraOnly := true
	for _, e := range childEntries {
		same, err := SameChildWrapperBaseVersion(e.OldVersion, e.NewVersion)
		if err != nil {
			return "", err
		}
		if !same {
			allExtraOnly = false
			break
		}
	}
	v, err := ParseChartVersion(current)
	if err != nil {
		return "", err
	}
	if allExtraOnly {
		v.Patch++
	} else {
		v.Minor++
	}
	return v.String(), nil
}

// NormalizeChartVersionEnv rewrites a Hydra chart version to the expected
// environment suffix for the chart directory. This keeps dev/stage root chart
// versions consistent even if a checked-in Chart.yaml lost its suffix.
func NormalizeChartVersionEnv(current, env string) (string, error) {
	v, err := ParseChartVersion(current)
	if err != nil {
		return "", err
	}
	switch env {
	case "", "prod":
		v.Env = ""
	default:
		v.Env = env
	}
	return v.String(), nil
}
