package hydra

import (
	"hydra-gitops.org/hydra/hydra-go/core/types"
	"k8s.io/apimachinery/pkg/util/sets"
)

// ApiVersionNormalizationDedupKey builds the key used to deduplicate
// "normalizing API version …" warnings for a cluster at runtime.
// gvk is group/version/kind (core resources: version/kind), same as entity.GVKString().
func ApiVersionNormalizationDedupKey(gvk string, preferredVersion types.Version) string {
	return gvk + "\t" + string(preferredVersion)
}

// HasLoggedApiVersionNormalization reports whether this warning was already logged
// for the given deduplication key on this cluster instance (in-memory only).
func (c *Cluster) HasLoggedApiVersionNormalization(key string) bool {
	if c == nil || key == "" {
		return false
	}
	c.apiNormWarnMu.Lock()
	defer c.apiNormWarnMu.Unlock()
	if c.apiNormWarnKeys == nil {
		c.apiNormWarnKeys = sets.New[string]()
	}
	return c.apiNormWarnKeys.Has(key)
}

// RememberLoggedApiVersionNormalization records that the warning for key was emitted
// on this cluster instance. State is not persisted across processes.
func (c *Cluster) RememberLoggedApiVersionNormalization(key string) {
	if c == nil || key == "" {
		return
	}
	c.apiNormWarnMu.Lock()
	defer c.apiNormWarnMu.Unlock()
	if c.apiNormWarnKeys == nil {
		c.apiNormWarnKeys = sets.New[string]()
	}
	c.apiNormWarnKeys.Insert(key)
}
