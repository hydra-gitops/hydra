package hydra

import (
	"hydra-gitops.org/hydra/hydra-go/base/errors"
	"hydra-gitops.org/hydra/hydra-go/base/log"
	"hydra-gitops.org/hydra/hydra-go/core/helm"
)

// ChartDirectoryForHydraApp returns the on-disk Helm chart directory used to render this app.
func ChartDirectoryForHydraApp(h HydraApp) (helm.ChartDirectory, error) {
	// ChildApp embeds *RootApp; check AsChildApp before AsRootApp.
	if c := h.AsChildApp(); c != nil {
		return c.ChildAppPath()
	}
	if r := h.AsRootApp(); r != nil {
		return r.RootAppPath(), nil
	}
	return nil, log.CreateError(errors.ErrInvalidHydraStructure, "ChartDirectoryForHydraApp requires a root or child app")
}

// ChartCacheForHydraApp returns the chart cache used when loading charts for this app.
func ChartCacheForHydraApp(h HydraApp) *helm.ChartCache {
	if r := h.AsRootApp(); r != nil {
		return r.ChartCache()
	}
	if c := h.AsChildApp(); c != nil {
		return c.ChartCache()
	}
	return nil
}
