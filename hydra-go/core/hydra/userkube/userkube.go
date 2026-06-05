// Package userkube loads optional per-user Hydra settings from the XDG config directory.
package userkube

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// UserKubeConfig is the root document of hydra/config.yaml under the XDG config directory.
type UserKubeConfig struct {
	Contexts []ContextMapping `yaml:"contexts"`
}

// ContextMapping maps a repository cluster directory to a kubeconfig file and context name.
type ContextMapping struct {
	Path   string `yaml:"path"`
	Config string `yaml:"config"`
	Name   string `yaml:"name"`
}

// DefaultConfigFilePath returns $XDG_CONFIG_HOME/hydra/config.yaml, or $HOME/.config/hydra/config.yaml
// when XDG_CONFIG_HOME is unset or empty.
func DefaultConfigFilePath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	base := os.Getenv("XDG_CONFIG_HOME")
	if base == "" {
		base = filepath.Join(home, ".config")
	}
	return filepath.Join(base, "hydra", "config.yaml"), nil
}

// ReadOptionalFile reads and parses the config file. If the file does not exist, it returns (nil, nil).
// Other read errors and YAML parse errors are returned as non-nil error.
func ReadOptionalFile(path string) (*UserKubeConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var cfg UserKubeConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}

// InvalidContextPath describes a contexts[].path entry that is set but does not refer to an existing directory.
type InvalidContextPath struct {
	YAMLPath string // path string as given in YAML
	Resolved string // absolute path after Clean/Abs, or empty if Abs failed
	Detail   string // short reason for logs
}

// InvalidContextPaths returns every non-empty contexts[].path that is not an existing directory after
// filepath.Clean and filepath.Abs (relative paths resolve against the current working directory).
func (c *UserKubeConfig) InvalidContextPaths() []InvalidContextPath {
	if c == nil {
		return nil
	}
	var out []InvalidContextPath
	for _, e := range c.Contexts {
		if e.Path == "" {
			continue
		}
		resolved, err := filepath.Abs(filepath.Clean(e.Path))
		if err != nil {
			out = append(out, InvalidContextPath{YAMLPath: e.Path, Resolved: "", Detail: err.Error()})
			continue
		}
		st, err := os.Stat(resolved)
		if err != nil {
			detail := err.Error()
			if errors.Is(err, fs.ErrNotExist) {
				detail = "does not exist"
			}
			out = append(out, InvalidContextPath{YAMLPath: e.Path, Resolved: resolved, Detail: detail})
			continue
		}
		if !st.IsDir() {
			out = append(out, InvalidContextPath{
				YAMLPath: e.Path,
				Resolved: resolved,
				Detail:   "not a directory",
			})
		}
	}
	return out
}

// InvalidKubeconfigPath describes a contexts[].config entry that is set but does not refer to an existing file.
type InvalidKubeconfigPath struct {
	YAMLConfig string // config string as given in YAML
	Resolved   string // absolute path after Clean/Abs, or empty if Abs failed
	Detail     string // short reason for logs
}

// InvalidKubeconfigPaths returns every non-empty contexts[].config that is not an existing non-directory
// path after filepath.Clean and filepath.Abs (relative paths resolve against the current working directory).
func (c *UserKubeConfig) InvalidKubeconfigPaths() []InvalidKubeconfigPath {
	if c == nil {
		return nil
	}
	var out []InvalidKubeconfigPath
	for _, e := range c.Contexts {
		if e.Config == "" {
			continue
		}
		resolved, err := filepath.Abs(filepath.Clean(e.Config))
		if err != nil {
			out = append(out, InvalidKubeconfigPath{YAMLConfig: e.Config, Resolved: "", Detail: err.Error()})
			continue
		}
		st, err := os.Stat(resolved)
		if err != nil {
			detail := err.Error()
			if errors.Is(err, fs.ErrNotExist) {
				detail = "does not exist"
			}
			out = append(out, InvalidKubeconfigPath{YAMLConfig: e.Config, Resolved: resolved, Detail: detail})
			continue
		}
		if st.IsDir() {
			out = append(out, InvalidKubeconfigPath{
				YAMLConfig: e.Config,
				Resolved:   resolved,
				Detail:     "is a directory, expected a kubeconfig file",
			})
		}
	}
	return out
}

// canonicalMatchPath returns an absolute path, optionally following a final symlink target when
// EvalSymlinks succeeds, for stable comparisons between config entries and runtime cluster paths.
func canonicalMatchPath(p string) (string, error) {
	a, err := filepath.Abs(filepath.Clean(p))
	if err != nil {
		return "", err
	}
	if sym, err := filepath.EvalSymlinks(a); err == nil {
		return filepath.Clean(sym), nil
	}
	return filepath.Clean(a), nil
}

// KubeMappingForClusterDir returns kubeconfig path and context name when an entry matches clusterDir.
// clusterDir should be the cluster directory (<hydra-context>/<clusterName>). Matching rules, in order:
//  1. Exact path match after Abs/Clean and optional EvalSymlinks on both sides.
//  2. Else: entry path equals the parent directory of clusterDir (Hydra context root), so one entry can
//     cover all clusters (e.g. in-cluster) under the same context directory.
//
// The first matching entry in each pass wins (file order).
//
// matchedViaHydraContextPath is true when rule (2) matched: the YAML path is the Hydra context directory,
// not the cluster directory. Callers should warn operators to prefer an explicit cluster path in config.
func (c *UserKubeConfig) KubeMappingForClusterDir(clusterDir string) (kubeconfigPath, contextName string, ok bool, matchedViaHydraContextPath bool) {
	if c == nil {
		return "", "", false, false
	}
	want, err := canonicalMatchPath(clusterDir)
	if err != nil {
		return "", "", false, false
	}
	for _, e := range c.Contexts {
		if e.Path == "" || e.Config == "" || e.Name == "" {
			continue
		}
		entryPath, perr := canonicalMatchPath(e.Path)
		if perr != nil {
			continue
		}
		if entryPath == want {
			return e.Config, e.Name, true, false
		}
	}
	parent, perr := canonicalMatchPath(filepath.Dir(want))
	if perr != nil {
		return "", "", false, false
	}
	for _, e := range c.Contexts {
		if e.Path == "" || e.Config == "" || e.Name == "" {
			continue
		}
		entryPath, perr := canonicalMatchPath(e.Path)
		if perr != nil {
			continue
		}
		if entryPath == parent {
			return e.Config, e.Name, true, true
		}
	}
	return "", "", false, false
}
