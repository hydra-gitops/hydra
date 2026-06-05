package ci

// NextChildChartWrapperVersion computes the next Helm wrapper version for a
// child chart from the dependency semver and environment. If the chart already
// carries the canonical wrapper for that dependency and environment, an extra
// counter is applied (e.g. 1.2.3-dev → 1.2.3-1-dev).
func NextChildChartWrapperVersion(depVer, env, currentVersion string) (string, error) {
	baseNew, err := ComputeWrapperVersion(depVer, env, -1)
	if err != nil {
		return "", err
	}
	if currentVersion == baseNew {
		return ComputeWrapperVersion(depVer, env, 0)
	}
	cur, err := ParseChartVersion(currentVersion)
	if err != nil {
		return baseNew, nil
	}
	want, err := ParseChartVersion(baseNew)
	if err != nil {
		return baseNew, nil
	}
	if cur.Major == want.Major && cur.Minor == want.Minor && cur.Patch == want.Patch &&
		cur.PreRelease == want.PreRelease && cur.Env == want.Env {
		if cur.Extra < 0 {
			return ComputeWrapperVersion(depVer, env, 0)
		}
		return ComputeWrapperVersion(depVer, env, cur.Extra)
	}
	return baseNew, nil
}
