package ci

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"gopkg.in/yaml.v3"
)

func WriteConfig(path string, cfg *Config) error {
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("marshaling config: %w", err)
	}
	return os.WriteFile(path, data, 0644)
}

func DetectAppGroups(baseDir string) ([]AppGroup, error) {
	if baseDir == "" {
		return nil, nil
	}
	entries, err := os.ReadDir(baseDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("reading directory %s: %w", baseDir, err)
	}
	var groups []AppGroup
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		groups = append(groups, AppGroup{
			Name: e.Name(),
			Path: filepath.Join(baseDir, e.Name()),
		})
	}
	sort.Slice(groups, func(i, j int) bool {
		return groups[i].Name < groups[j].Name
	})
	return groups, nil
}

func DetectRootApps(appGroups []AppGroup) ([]string, error) {
	var rootApps []string
	for _, g := range appGroups {
		rootDir := filepath.Join(g.Path, "root")
		info, err := os.Stat(rootDir)
		if err != nil || !info.IsDir() {
			continue
		}
		entries, err := os.ReadDir(rootDir)
		if err != nil {
			continue
		}
		for _, e := range entries {
			if e.IsDir() {
				rootApps = append(rootApps, g.Name)
				break
			}
		}
	}
	return rootApps, nil
}

// DetectEnvironments scans rootAppsPath/*/*/ for subdirectory names
// (i.e. rootAppsPath/<group>/<app>/<env>) and returns the unique,
// sorted set of environment names found.
func DetectEnvironments(rootAppsPath string) ([]string, error) {
	if rootAppsPath == "" {
		return nil, nil
	}
	pattern := filepath.Join(rootAppsPath, "*", "*", "*")
	matches, err := filepath.Glob(pattern)
	if err != nil {
		return nil, fmt.Errorf("glob %s: %w", pattern, err)
	}
	envSet := make(map[string]struct{})
	for _, m := range matches {
		info, err := os.Stat(m)
		if err == nil && info.IsDir() {
			envSet[filepath.Base(m)] = struct{}{}
		}
	}
	var envs []string
	for name := range envSet {
		envs = append(envs, name)
	}
	sort.Strings(envs)
	return envs, nil
}

func DefaultConfig() *Config {
	return &Config{
		CI: CIConfig{
			RootAppsPath: "apps",
			Environments: []string{"dev", "stage", "prod"},
		},
	}
}

func ValidateOutputPath(path string) error {
	info, err := os.Stat(path)
	if err == nil {
		if info.IsDir() {
			return fmt.Errorf("path %q is a directory", path)
		}
		return nil
	}
	parent := filepath.Dir(path)
	parentInfo, err := os.Stat(parent)
	if err != nil || !parentInfo.IsDir() {
		return fmt.Errorf("parent directory %q does not exist", parent)
	}
	return nil
}
