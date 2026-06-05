package hydra

import (
	"path/filepath"

	"hydra-gitops.org/hydra/hydra-go/base/log"

	"hydra-gitops.org/hydra/hydra-go/base/errors"
	"hydra-gitops.org/hydra/hydra-go/core/hydra/userkube"
	"hydra-gitops.org/hydra/hydra-go/core/k8s"
	"hydra-gitops.org/hydra/hydra-go/core/types"
	"k8s.io/cli-runtime/pkg/genericclioptions"
)

func validateClusterName(expected, given types.ClusterName) error {
	if expected != given {
		return log.CreateError(
			errors.ErrHydraContextProblem,
			"cluster name mismatch: expected '{expected}' but got '{given}'",
			log.String("expected", string(expected)),
			log.String("given", string(given)))
	}
	return nil
}

// validateCluster checks if the app's cluster matches the provided cluster name
func validateCluster(appId types.AppId, clusterName types.ClusterName) error {
	appClusterName, err := appId.ClusterName()
	if err != nil {
		return err
	}

	if err := validateClusterName(clusterName, appClusterName); err != nil {
		return err
	}

	return nil
}

// validateRootApp checks if the app's rootApp matches the provided rootApp name.
// It first validates the cluster by calling validateCluster.
func validateRootApp(
	appId types.AppId,
	clusterName types.ClusterName,
	rootAppName types.RootAppName,
) error {
	// First validate the cluster
	if err := validateCluster(appId, clusterName); err != nil {
		return err
	}

	appRootAppName, err := appId.RootAppName()
	if err != nil {
		return err
	}

	if rootAppName != appRootAppName {
		return log.CreateError(
			errors.ErrInvalidHydraStructure,
			"rootApp mismatch: app references rootApp '{app_rootApp}' but context resolved to '{resolved_rootApp}'",
			log.String("app_rootApp", string(appRootAppName)),
			log.String("resolved_rootApp", string(rootAppName)))
	}

	return nil
}

// validateChildApp checks if the app's childApp matches the provided childApp name.
// It first validates the rootApp (which also validates the cluster) by calling validateRootApp.
func validateChildApp(
	appId types.AppId,
	clusterName types.ClusterName,
	rootAppName types.RootAppName,
	childAppName types.ChildAppName,
) error {
	// First validate the root app (which validates cluster)
	if err := validateRootApp(appId, clusterName, rootAppName); err != nil {
		return err
	}

	appChildAppName, err := appId.ChildAppName()
	if err != nil {
		return err
	}

	if appChildAppName == nil {
		return log.CreateError(
			errors.ErrInvalidHydraStructure,
			"childApp mismatch: app does not specify a childApp but context resolved to '{resolved_childApp}'",
			log.String("resolved_childApp", string(childAppName)))
	}

	if childAppName != *appChildAppName {
		return log.CreateError(
			errors.ErrInvalidHydraStructure,
			"childApp mismatch: app references childApp '{app_childApp}' but context resolved to '{resolved_childApp}'",
			log.String("app_childApp", string(*appChildAppName)),
			log.String("resolved_childApp", string(childAppName)))
	}

	return nil
}

// BuildValidatedKubernetesConfigFlagsForCluster returns client-go config flags after applying an optional
// XDG user mapping (see userkube package) and validating allowed kubectl contexts for the cluster.
func BuildValidatedKubernetesConfigFlagsForCluster(cluster *Cluster) (*genericclioptions.ConfigFlags, error) {
	if cluster == nil {
		return nil, log.CreateError(
			errors.ErrHydraContextProblem,
			"kubernetes context validation can not be performed without a cluster")
	}

	hydraValues, err := HydraValues(cluster, types.HelmNetworkModeOffline)
	if err != nil {
		return nil, err
	}

	color := cluster.Config().Color()
	configFlags := genericclioptions.NewConfigFlags(true)

	configPath, pathErr := userkube.DefaultConfigFilePath()
	if pathErr != nil {
		cluster.L().Warn(logIdCluster, "could not resolve Hydra user config path: {err}", log.Err(pathErr))
	} else {
		userCfg, readErr := userkube.ReadOptionalFile(configPath)
		if readErr != nil {
			cluster.L().Warn(logIdCluster, "failed to read Hydra user kubeconfig mapping file {path}: {err}",
				log.String("path", configPath),
				log.Err(readErr))
		} else if userCfg != nil {
			for _, inv := range userCfg.InvalidContextPaths() {
				cluster.L().Warn(logIdCluster,
					"Hydra user config {userConfig}: contexts[].path {yamlPath} (resolved {resolved}) is not a usable directory: {detail}",
					log.String("userConfig", configPath),
					log.String("yamlPath", inv.YAMLPath),
					log.String("resolved", inv.Resolved),
					log.String("detail", inv.Detail))
			}
			for _, inv := range userCfg.InvalidKubeconfigPaths() {
				cluster.L().Warn(logIdCluster,
					"Hydra user config {userConfig}: contexts[].config {yamlConfig} (resolved {resolved}) is not a usable kubeconfig file: {detail}",
					log.String("userConfig", configPath),
					log.String("yamlConfig", inv.YAMLConfig),
					log.String("resolved", inv.Resolved),
					log.String("detail", inv.Detail))
			}
			clusterDir, absErr := filepath.Abs(filepath.Clean(cluster.ClusterPath()))
			if absErr != nil {
				return nil, absErr
			}
			if kubeFile, ctxName, ok, viaHydraContextPath := userCfg.KubeMappingForClusterDir(clusterDir); ok {
				if viaHydraContextPath {
					cluster.L().Warn(logIdCluster,
						"Hydra user config {userConfig}: contexts[].path should be the cluster directory (e.g. {clusterDir}), not the Hydra context root. A context-level path matched; update path for clarity and to avoid accidental reuse across clusters.",
						log.String("userConfig", configPath),
						log.String("clusterDir", clusterDir))
				}
				kubePath := kubeFile
				ctx := ctxName
				configFlags.KubeConfig = &kubePath
				configFlags.Context = &ctx
				if string(cluster.ContextPath) != "" {
					cluster.L().Info(logIdCluster,
						"using kube config from user config file {userConfig} (kubectl context {kubectlContext}, kubeconfig {kubeconfig})",
						log.String("userConfig", configPath),
						log.String("kubectlContext", ctxName),
						log.String("kubeconfig", kubeFile))
				}
			}
		}
	}

	if err := k8s.ValidateApiContext(cluster.L(), color, hydraValues, cluster.Description(), configFlags); err != nil {
		return nil, err
	}

	return configFlags, nil
}

// HydraClusterAccess validates the Kubernetes context and returns config flags
func HydraClusterAccess(h Hydra) (*genericclioptions.ConfigFlags, error) {
	if h.AsCluster() == nil {
		return nil, log.CreateError(
			errors.ErrHydraContextProblem,
			"kubernetes context validation can not be performed on hydra context level")
	}

	if h.Config().KubernetesConnectionAllowed() == types.KubernetesConnectionAllowedNo {
		return nil, log.CreateError(
			errors.ErrKubernetesConnectionNotAllowed,
			"kubernetes connection is not allowed in this hydra session")
	}

	cluster := h.AsCluster()
	cluster.kubeRESTFlagsOnce.Do(func() {
		cluster.kubeRESTFlags, cluster.kubeRESTFlagsErr = BuildValidatedKubernetesConfigFlagsForCluster(cluster)
	})
	return cluster.kubeRESTFlags, cluster.kubeRESTFlagsErr
}
