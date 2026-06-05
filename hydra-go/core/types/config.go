package types

// Config is an interface for Hydra configuration options.
type Config interface {
	Color() Color
	DryRun() DryRun
	KubernetesConnectionAllowed() KubernetesConnectionAllowed
	HelmTemplateCacheEnabled() bool
}

// config holds configuration options for the Hydra context.
type config struct {
	color                       Color
	dryRun                      DryRun
	kubernetesConnectionAllowed KubernetesConnectionAllowed
	helmTemplateCacheEnabled    bool
}

// Ensure config implements Config interface
var _ Config = (*config)(nil)

func (c *config) Color() Color {
	return c.color
}

func (c *config) DryRun() DryRun {
	return c.dryRun
}

func (c *config) KubernetesConnectionAllowed() KubernetesConnectionAllowed {
	return c.kubernetesConnectionAllowed
}

func (c *config) HelmTemplateCacheEnabled() bool {
	return c.helmTemplateCacheEnabled
}

// NewConfig creates a new Config with the given options.
func NewConfig(
	color Color,
	dryRun DryRun,
	kubernetesConnectionAllowed KubernetesConnectionAllowed,
	helmTemplateCacheEnabled bool,
) Config {
	return &config{
		color:                       color,
		dryRun:                      dryRun,
		kubernetesConnectionAllowed: kubernetesConnectionAllowed,
		helmTemplateCacheEnabled:    helmTemplateCacheEnabled,
	}
}
