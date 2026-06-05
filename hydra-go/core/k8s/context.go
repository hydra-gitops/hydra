package k8s

import (
	"hydra-gitops.org/hydra/hydra-go/base/log"

	"hydra-gitops.org/hydra/hydra-go/base/errors"
	"hydra-gitops.org/hydra/hydra-go/core/types"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/clientcmd/api"
)

// CurrentContext retrieves the current kubectl context using the kubectl Go API.
func CurrentContext() (*types.HydraKubectlContext, error) {
	config, err := clientcmd.NewDefaultClientConfigLoadingRules().Load()
	if err != nil {
		return nil, log.CreateError(
			errors.ErrKubeConfig,
			"error loading kubeconfig: {err}",
			log.Err(err))
	}
	return Context(config)
}

func Context(config *api.Config) (*types.HydraKubectlContext, error) {
	if config.CurrentContext == "" {
		return nil, log.CreateError(
			errors.ErrKubeConfig,
			"no current context set in kubeconfig")
	}

	for name, context := range config.Contexts {
		if name == config.CurrentContext {
			return &types.HydraKubectlContext{
				Name:     name,
				Cluster:  context.Cluster,
				AuthInfo: context.AuthInfo,
			}, nil
		}
	}

	return nil,
		log.CreateError(
			errors.ErrKubeConfig,
			"context '{context}' not found in kubeconfig",
			log.String("context", config.CurrentContext))
}
