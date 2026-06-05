package k8s

import (
	"hydra-gitops.org/hydra/hydra-go/base/log"

	"hydra-gitops.org/hydra/hydra-go/base/errors"
	"hydra-gitops.org/hydra/hydra-go/core/types"
	"hydra-gitops.org/hydra/hydra-go/core/yq"
	"k8s.io/cli-runtime/pkg/genericclioptions"
)

// ValidateCurrentContext validates that the current kubectl context is allowed for the Hydra implementation.
// It checks if the current kubectl context is in the list of allowed contexts.
// Returns an error if validation fails or if no hydra configuration is found.
func ValidateCurrentContext(l log.Logger, color types.Color, hydraValues *types.HydraValues, description string) error {
	context, err := CurrentContext()
	if err != nil {
		return log.CreateError(
			errors.ErrKubeConfig,
			"error retrieving current kubectl context",
			log.Err(err))
	}
	return ValidateContext(l, color, hydraValues, description, context)
}

func ValidateApiContext(
	l log.Logger,
	color types.Color,
	hydraValues *types.HydraValues,
	description string,
	configFlags *genericclioptions.ConfigFlags,
) error {
	config, err := configFlags.ToRawKubeConfigLoader().RawConfig()
	if err != nil {
		return err
	}
	if configFlags.Context != nil && *configFlags.Context != "" {
		config.CurrentContext = *configFlags.Context
	}
	hcontext, err := Context(&config)
	if err != nil {
		return err
	}
	return ValidateContext(l, color, hydraValues, description, hcontext)
}

func ValidateContext(
	l log.Logger,
	color types.Color,
	hydraValues *types.HydraValues,
	description string,
	currentContext *types.HydraKubectlContext,
) error {
	// Hydra configuration must exist
	if hydraValues == nil {
		return log.CreateError(errors.ErrHydraConfigError, "no hydra configuration found")
	}

	// AllowedContexts must not be empty
	if len(hydraValues.KubeCtl.AllowedContexts) == 0 {
		hydraYaml, err := yq.ToYaml(color, hydraValues)
		if err == nil {
			l.DebugLog(logIdK8s, "hydra values for {description}:\n{hydra}",
				log.String("description", description),
				log.String("hydra", string(hydraYaml)))
		} else {
			l.Warn(logIdK8s, "failed to convert hydra configuration to YAML for logging",
				log.Err(err))
		}
		return log.CreateError(errors.ErrHydraConfigError,
			"please add global.hydra.kubectl.allowedContexts to your hydra configuration")
	}

	// Check if the current context is in the list of allowed contexts
	for _, allowed := range hydraValues.KubeCtl.AllowedContexts {
		if allowed.Name != "" && allowed.Name != currentContext.Name {
			continue
		}
		if allowed.AuthInfo != "" && allowed.AuthInfo != currentContext.AuthInfo {
			continue
		}
		if allowed.Cluster != "" && allowed.Cluster != currentContext.Cluster {
			continue
		}
		return nil
	}

	allowedYaml, err := yq.ToYaml(color, hydraValues.KubeCtl.AllowedContexts)
	if err != nil {
		return log.CreateError(
			errors.ErrHydraConfigError,
			"failed to convert allowed contexts to YAML",
			log.Err(err))
	}

	// Error if context is not allowed
	return log.CreateError(
		errors.ErrHydraContextProblem,
		"current kubectl context '{context}' is not allowed. Allowed contexts:\n{allowed}",
		log.String("context", currentContext.Name),
		log.Any("allowed", allowedYaml))
}
