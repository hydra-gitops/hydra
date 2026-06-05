package types

import (
	"fmt"
	"strings"

	"hydra-gitops.org/hydra/hydra-go/base/errors"
	"hydra-gitops.org/hydra/hydra-go/base/log"
)

type ClusterName string
type RootAppName string
type ChildAppName string

// ReservedPresetRootAppName is the root app segment reserved for synthetic preset apps
// ({cluster}.preset.{presetId}). Declared Hydra root app directories must not use this name.
const ReservedPresetRootAppName RootAppName = "preset"

func NewRootAppId(clusterName ClusterName, rootAppName RootAppName) AppId {
	return AppId(fmt.Sprintf("%s.%s", clusterName, rootAppName))
}

func NewChildAppId(clusterName ClusterName, rootAppName RootAppName, childAppName ChildAppName) AppId {
	return AppId(fmt.Sprintf("%s.%s.%s", clusterName, rootAppName, childAppName))
}

func NewClusterName(name string) (ClusterName, error) {
	if strings.Contains(name, ".") {
		return "", log.CreateError(errors.ErrHydraConfigError,
			"invalid cluster name '{name}': name must not contain '.'",
			log.String("name", name))
	}
	return ClusterName(name), nil
}

// AppIdPattern is a CLI input that may contain a trailing * wildcard.
// It must be resolved to one or more concrete AppId values before use.
type AppIdPattern string

// ToAppIdPatterns converts a slice of strings (e.g. cobra args) to AppIdPatterns.
func ToAppIdPatterns(args []string) []AppIdPattern {
	patterns := make([]AppIdPattern, len(args))
	for i, a := range args {
		patterns[i] = AppIdPattern(a)
	}
	return patterns
}

// AppId is a string type representing a resolved app reference in the format:
// <cluster>.<rootApp>.<childApp> where childApp is optional.
// An AppId never contains wildcards.
type AppId string

// NewAppId creates a new App from a string and validates the format
// The format must be <cluster>.<rootApp> or <cluster>.<rootApp>.<childApp>
func NewAppId(appId string) (AppId, error) {
	parts := strings.Split(appId, ".")
	if len(parts) < 2 || len(parts) > 3 {
		return "", log.CreateError(errors.ErrHydraConfigError,
			"invalid app id format: expected <cluster>.<rootApp> or <cluster>.<rootApp>.<childApp>, got '{app}'",
			log.String("app", appId))
	}

	// Validate that no part is empty
	for i, part := range parts {
		if part == "" {
			return "", log.CreateError(
				errors.ErrHydraConfigError,
				"invalid app id format: empty part at position {position} in '{app}'",
				log.Int("position", i),
				log.String("app", appId))
		}
	}

	return AppId(appId), nil
}

// IsPresetApp reports synthetic cluster-defaults preset ownership ids of the form
// <cluster>.preset.<presetId> (three dot-separated segments with middle segment "preset").
func (a AppId) IsPresetApp() bool {
	parts := strings.Split(string(a), ".")
	if len(parts) != 3 {
		return false
	}
	return parts[1] == string(ReservedPresetRootAppName)
}

// NewPresetAppId builds the synthetic app id for cluster-defaults preset ownership.
// presetID must be non-empty, must not contain '.', and must match [a-z0-9-]+ (preset file ids).
func NewPresetAppId(cluster ClusterName, presetID string) (AppId, error) {
	if presetID == "" {
		return "", log.CreateError(errors.ErrHydraConfigError,
			"preset app: preset id must not be empty")
	}
	if strings.Contains(presetID, ".") {
		return "", log.CreateError(errors.ErrHydraConfigError,
			"preset app: preset id must not contain '.'",
			log.String("presetId", presetID))
	}
	for _, r := range presetID {
		if (r < 'a' || r > 'z') && (r < '0' || r > '9') && r != '-' {
			return "", log.CreateError(errors.ErrHydraConfigError,
				"preset app: preset id must use only [a-z0-9-]",
				log.String("presetId", presetID))
		}
	}
	return NewChildAppId(cluster, ReservedPresetRootAppName, ChildAppName(presetID)), nil
}

// IsRootApp returns true if the AppId refers to a root app (format: <cluster>.<rootApp>)
func (a AppId) IsRootApp() bool {
	return len(strings.Split(string(a), ".")) == 2
}

func ExpectRootAppId(appId AppId) error {
	parts := strings.Split(string(appId), ".")
	if len(parts) != 2 {
		return log.CreateError(
			errors.ErrHydraConfigError,
			"invalid root app id format: expected <cluster>.<rootApp>, got '{appId}'",
			log.String("appId", string(appId)))
	}
	return nil
}

// ClusterName returns the cluster name from the app reference
// Format: <cluster>.<rootApp> or <cluster>.<rootApp>.<childApp>
func (a AppId) ClusterName() (ClusterName, error) {
	parts := strings.Split(string(a), ".")
	if len(parts) < 2 || len(parts) > 3 {
		return "", log.CreateError(
			errors.ErrHydraConfigError,
			"invalid app id format: expected <cluster>.<rootApp> or <cluster>.<rootApp>.<childApp>, got '{app}'",
			log.String("app", string(a)))
	}
	return ClusterName(parts[0]), nil
}

// RootAppName returns the root app name from the app reference
// Format: <cluster>.<rootApp> or <cluster>.<rootApp>.<childApp>
func (a AppId) RootAppName() (RootAppName, error) {
	parts := strings.Split(string(a), ".")
	if len(parts) < 2 || len(parts) > 3 {
		return "", log.CreateError(
			errors.ErrHydraConfigError,
			"invalid app id format: expected <cluster>.<rootApp> or <cluster>.<rootApp>.<childApp>, got '{app}'",
			log.String("app", string(a)))
	}
	return RootAppName(parts[1]), nil
}

// ChildAppName returns the child app name from the app reference, or nil if not provided
// Format: <cluster>.<rootApp> or <cluster>.<rootApp>.<childApp>
func (a AppId) ChildAppName() (*ChildAppName, error) {
	parts := strings.Split(string(a), ".")
	if len(parts) < 2 || len(parts) > 3 {
		return nil, log.CreateError(
			errors.ErrHydraConfigError,
			"invalid app id format: expected <cluster>.<rootApp> or <cluster>.<rootApp>.<childApp>, got '{app}'",
			log.String("app", string(a)))
	}

	if len(parts) == 3 {
		childApp := ChildAppName(parts[2])
		return &childApp, nil
	}

	return nil, nil
}
