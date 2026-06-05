package hydra

// Kubernetes annotation keys for Hydra. Keys must be a single optional DNS prefix and one name
// segment separated by one '/' (see IsQualifiedName); extra slashes are invalid. The name segment
// encodes the module identity (hydracd/hydra/...) with hyphens instead of path slashes.
const (
	AnnotationHydraConfig        = "hydra-gitops.org/hydra-config"
	AnnotationHydraBackup        = "hydra-gitops.org/hydra-backup"
	AnnotationHydraScaleDisabled = "hydra-gitops.org/hydra-disabled"
	// AnnotationHydraCloneSource marks resources materialized by global.hydra.clones (value: declaringAppId/ruleName).
	AnnotationHydraCloneSource = "hydra-gitops.org/hydra-clone-source"
)
