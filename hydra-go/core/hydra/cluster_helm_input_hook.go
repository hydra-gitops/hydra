package hydra

import (
	"hydra-gitops.org/hydra/hydra-go/core/types"
)

// SetHelmInputValuesForApp registers a callback that supplies Helm chart input values per app ID.
// When set, [RootApp.Template] and [ChildApp.Template] bypass template caches and use these values
// instead of [HydraApp.LoadValuesMap] for the Helm render path. Call [Cluster.ClearHelmInputValuesForApp]
// in a defer when finished (including on error paths).
func (c *Cluster) SetHelmInputValuesForApp(f func(types.AppId) (types.ValuesMap, error)) {
	if c == nil {
		return
	}
	c.helmInputMu.Lock()
	c.helmInputValuesForApp = f
	c.helmInputMu.Unlock()
}

// ClearHelmInputValuesForApp removes the per-app Helm values hook installed by [Cluster.SetHelmInputValuesForApp].
func (c *Cluster) ClearHelmInputValuesForApp() {
	if c == nil {
		return
	}
	c.helmInputMu.Lock()
	c.helmInputValuesForApp = nil
	c.helmInputMu.Unlock()
}

func (c *Cluster) helmChartInputValues(appId types.AppId) (types.ValuesMap, bool, error) {
	if c == nil {
		return nil, false, nil
	}
	c.helmInputMu.Lock()
	fn := c.helmInputValuesForApp
	c.helmInputMu.Unlock()
	if fn == nil {
		return nil, false, nil
	}
	v, err := fn(appId)
	return v, true, err
}
