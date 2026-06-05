package types

import (
	"os"
	"strings"
)

// HelmTemplateCacheDisabledByEnv reports whether HYDRA_NO_CACHE requests cache bypass.
func HelmTemplateCacheDisabledByEnv() bool {
	raw := strings.TrimSpace(os.Getenv(HydraNoCacheEnvName))
	if raw == "" {
		return false
	}
	switch strings.ToLower(raw) {
	case "1", "true", "t", "yes", "y", "on":
		return true
	default:
		return false
	}
}
