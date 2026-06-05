package ci

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"hydra-gitops.org/hydra/hydra-go/core/git"
)

// envDir returns the environment directory segment under apps/<group>/<app>/.
func envDir(env string) string {
	if env == "" {
		return "prod"
	}
	return env
}

func sortGroupsByNameLengthDesc(groups []string) []string {
	out := append([]string(nil), groups...)
	sort.Slice(out, func(i, j int) bool {
		if len(out[i]) != len(out[j]) {
			return len(out[i]) > len(out[j])
		}
		return out[i] < out[j]
	})
	return out
}

// resolvePackageGroupNames returns app group names for tag parsing: configured
// appGroups first; if none, unique groups discovered from chart directories.
func resolvePackageGroupNames(cfg *Config, repo *git.Repo) ([]string, error) {
	seen := make(map[string]struct{})
	var names []string
	for _, ag := range cfg.CI.AppGroups {
		if ag.Name == "" {
			continue
		}
		if _, ok := seen[ag.Name]; ok {
			continue
		}
		seen[ag.Name] = struct{}{}
		names = append(names, ag.Name)
	}
	if len(names) > 0 {
		return sortGroupsByNameLengthDesc(names), nil
	}
	return discoverGroupNamesFromChartTree(repo, cfg.CI.RootAppsPath)
}

func discoverGroupNamesFromChartTree(repo *git.Repo, rootAppsPath string) ([]string, error) {
	pattern := filepath.Join(repo.Path(), rootAppsPath, "*", "*", "*")
	matches, err := filepath.Glob(pattern)
	if err != nil {
		return nil, fmt.Errorf("glob chart dirs: %w", err)
	}
	seen := make(map[string]struct{})
	for _, abs := range matches {
		rel, err := filepath.Rel(repo.Path(), abs)
		if err != nil {
			continue
		}
		rel = filepath.ToSlash(rel)
		group, _, _, err := ParseChartPath(rel)
		if err != nil {
			continue
		}
		if group == "" {
			continue
		}
		seen[group] = struct{}{}
	}
	var out []string
	for g := range seen {
		out = append(out, g)
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("no app groups in ci.appGroups and no charts under %s", rootAppsPath)
	}
	return sortGroupsByNameLengthDesc(out), nil
}

// chartRelPathFromReleaseTag maps a release documentation tag to a chart path
// under rootAppsPath. Returns ok false for build tags or unrecognized tags.
func chartRelPathFromReleaseTag(tag, rootAppsPath string, groups []string) (rel string, ok bool, err error) {
	if strings.HasPrefix(tag, BuildTagPrefix) {
		return "", false, nil
	}
	groups = sortGroupsByNameLengthDesc(groups)
	for _, g := range groups {
		rootPrefix := g + "-root-"
		if strings.HasPrefix(tag, rootPrefix) {
			verStr := strings.TrimPrefix(tag, rootPrefix)
			cv, err := ParseChartVersion(verStr)
			if err != nil {
				return "", false, nil
			}
			p := filepath.ToSlash(filepath.Join(rootAppsPath, g, "root", envDir(cv.Env)))
			return p, true, nil
		}
	}
	for _, g := range groups {
		prefix := g + "-"
		if !strings.HasPrefix(tag, prefix) {
			continue
		}
		rest := strings.TrimPrefix(tag, prefix)
		if rest == "" {
			continue
		}
		// Version strings contain dots (1.200.9); splitting rest on "-" breaks semver.
		// Take the leftmost index where rest[i:] parses as a Hydra chart version.
		bestIdx := -1
		var bestCV ChartVersion
		for i, ch := range rest {
			if ch < '0' || ch > '9' {
				continue
			}
			verCandidate := rest[i:]
			cv, err := ParseChartVersion(verCandidate)
			if err != nil {
				continue
			}
			app := strings.TrimSuffix(rest[:i], "-")
			if app == "" {
				continue
			}
			if bestIdx == -1 || i < bestIdx {
				bestIdx = i
				bestCV = cv
			}
		}
		if bestIdx >= 0 {
			app := strings.TrimSuffix(rest[:bestIdx], "-")
			p := filepath.ToSlash(filepath.Join(rootAppsPath, g, app, envDir(bestCV.Env)))
			return p, true, nil
		}
	}
	return "", false, nil
}
