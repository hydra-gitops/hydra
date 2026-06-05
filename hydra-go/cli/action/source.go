package action

import (
	"strings"

	"hydra-gitops.org/hydra/hydra-go/base/log"
	"hydra-gitops.org/hydra/hydra-go/cli/flags"
	"hydra-gitops.org/hydra/hydra-go/core/helm"
	"hydra-gitops.org/hydra/hydra-go/core/highlight"
	"hydra-gitops.org/hydra/hydra-go/core/hydra"
	"hydra-gitops.org/hydra/hydra-go/core/types"
)

const (
	sourceBannerUnrendered = "# hydra local source: Helm chart template sources (unrendered; not valid Kubernetes YAML).\n"
	sourceNoTemplates      = "# (no chart template files match the selection)\n"
)

// SourceFlags holds flags for hydra local source.
type SourceFlags struct {
	flags.ColorFlag
	flags.HelmNetworkModeFlag
	flags.ContextFlag
	flags.ExcludeAppFlag
	flags.NoCacheFlag
	flags.IncludePathFlag
	AppId types.AppId
}

var _ flags.Flags = (*SourceFlags)(nil)
var _ flags.WithColorFlag = (*SourceFlags)(nil)
var _ flags.WithContextFlag = (*SourceFlags)(nil)
var _ flags.WithExcludeAppFlag = (*SourceFlags)(nil)
var _ flags.WithHelmNetworkModeFlag = (*SourceFlags)(nil)
var _ flags.WithIncludePathFlag = (*SourceFlags)(nil)
var _ flags.WithNoCacheFlag = (*SourceFlags)(nil)

func (f *SourceFlags) Flags() flags.Flags {
	return f
}

func (f *SourceFlags) WithColorFlag() *flags.ColorFlag {
	return &f.ColorFlag
}

func (f *SourceFlags) WithContextFlag() *flags.ContextFlag {
	return &f.ContextFlag
}

func (f *SourceFlags) WithExcludeAppFlag() *flags.ExcludeAppFlag {
	return &f.ExcludeAppFlag
}

func (f *SourceFlags) WithHelmNetworkModeFlag() *flags.HelmNetworkModeFlag {
	return &f.HelmNetworkModeFlag
}

func (f *SourceFlags) WithIncludePathFlag() *flags.IncludePathFlag {
	return &f.IncludePathFlag
}

func (f *SourceFlags) WithNoCacheFlag() *flags.NoCacheFlag {
	return &f.NoCacheFlag
}

// Source prints unrendered Helm chart template bodies for the resolved app. Templates are taken
// from the loaded chart (including packaged charts/*.tgz dependencies), not only from loose
// templates/ directories on disk.
// Optional --include-path prefixes filter by Helm template path (OR semantics).
func Source(f SourceFlags) (hydra.Hydra, string, error) {
	l := log.Default()
	config := flags.NewConfigFromFlags(&f, types.KubernetesConnectionAllowedNo)
	h, err := resolveHydraAppForLocalPrint(l, f.HydraContext, f.AppId, config, f.HelmNetworkMode)
	if err != nil {
		return nil, "", err
	}
	app := h.AsApp()
	chartDir, err := hydra.ChartDirectoryForHydraApp(app)
	if err != nil {
		return nil, "", err
	}
	charter, err := chartDir.LoadChart(hydra.ChartCacheForHydraApp(app), f.HelmNetworkMode)
	if err != nil {
		return nil, "", err
	}
	prefixes := make([]string, 0, len(f.IncludePathPrefixes))
	for _, p := range f.IncludePathPrefixes {
		n := helm.NormalizeTemplateSourcePathPrefix(p)
		if n != "" {
			prefixes = append(prefixes, n)
		}
	}
	srcBlock, err := helm.ChartSourceTemplatesMultidoc(charter, prefixes)
	if err != nil {
		return nil, "", err
	}
	if strings.TrimSpace(srcBlock) == "" {
		srcBlock = sourceNoTemplates
	}
	body := srcBlock
	if f.Color {
		colored, err := highlight.GoTemplateTerminal256(f.Color, srcBlock)
		if err != nil {
			return nil, "", err
		}
		body = colored
	}
	out := sourceBannerUnrendered + body
	return h, out, nil
}
