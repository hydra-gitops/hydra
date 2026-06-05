package errors

type Error interface {
	error
	ErrorId() ErrorId
}

type ErrorId string

func Id(err error) ErrorId {
	if e, ok := err.(Error); ok {
		return e.ErrorId()
	}
	return ErrUnknown
}

func errorId(err error) ErrorId {
	if e, ok := err.(Error); ok {
		return e.ErrorId()
	}
	return ErrUnknown
}

func IsKnownError(err error) bool {
	return errorId(err) != ErrUnknown
}

const ErrUnknown ErrorId = ""

func (id ErrorId) MatchesError(err error) bool {
	return errorId(err) == id
}

const (
	ErrAborted                                    ErrorId = "ErrAborted"
	ErrBootstrapGuard                             ErrorId = "ErrBootstrapGuard"
	ErrAppIdIsNoRootApp                           ErrorId = "ErrAppIdIsNoRootApp"
	ErrAppIdsDifferentClusters                    ErrorId = "ErrAppIdsDifferentClusters"
	ErrAppNotEnabled                              ErrorId = "ErrAppNotEnabled"
	ErrAppNotFound                                ErrorId = "ErrAppNotFound"
	ErrCelCompileFailed                           ErrorId = "ErrCelCompileFailed"
	ErrConflictingPreferredApiVersions            ErrorId = "ErrConflictingPreferredApiVersions"
	ErrCelNewEnvFailed                            ErrorId = "ErrCelNewEnvFailed"
	ErrCelProgramFailed                           ErrorId = "ErrCelProgramFailed"
	ErrCiUpgrade                                  ErrorId = "ErrCiUpgrade"
	ErrCiDownload                                 ErrorId = "ErrCiDownload"
	ErrCiSprint                                   ErrorId = "ErrCiSprint"
	ErrCiSync                                     ErrorId = "ErrCiSync"
	ErrCiTest                                     ErrorId = "ErrCiTest"
	ErrCiUpdate                                   ErrorId = "ErrCiUpdate"
	ErrCrdGvkMismatch                             ErrorId = "ErrCrdGvkMismatch"
	ErrCreateTempDirFailed                        ErrorId = "ErrCreateTempDirFailed"
	ErrDidNotEvalToBool                           ErrorId = "ErrDidNotEvalToBool"
	ErrEvaluationFailed                           ErrorId = "ErrEvaluationFailed"
	ErrFailedToListResources                      ErrorId = "ErrFailedToListResources"
	ErrFailedToParseReplicas                      ErrorId = "ErrFailedToParseReplicas"
	ErrHelmTemplateFailed                         ErrorId = "ErrHelmTemplateFailed"
	ErrHydraConfigError                           ErrorId = "ErrHydraConfigError"
	ErrHydraContextProblem                        ErrorId = "ErrHydraContextProblem"
	ErrInternalError                              ErrorId = "ErrInternalError"
	ErrInvalidCrdMode                             ErrorId = "ErrInvalidCrdMode"
	ErrInvalidHydraStructure                      ErrorId = "ErrInvalidHydraStructure"
	ErrInvalidResourceScope                       ErrorId = "ErrInvalidResourceScope"
	ErrKeyNotFound                                ErrorId = "ErrKeyNotFound"
	ErrKeyTypeMismatch                            ErrorId = "ErrKeyTypeMismatch"
	ErrKubeConfig                                 ErrorId = "ErrKubeConfig"
	ErrKubeCtlApplyFailed                         ErrorId = "ErrKubeCtlApplyFailed"
	ErrKubernetesConnectionNotAllowed             ErrorId = "ErrKubernetesConnectionNotAllowed"
	ErrLoadingHelmChartDependenciesFailed         ErrorId = "ErrLoadingHelmChartDependenciesFailed"
	ErrLoadingHelmChartFailed                     ErrorId = "ErrLoadingHelmChartFailed"
	ErrMetadataError                              ErrorId = "ErrMetadataError"
	ErrMissingHydraContext                        ErrorId = "ErrMissingHydraContext"
	ErrMissingScope                               ErrorId = "ErrMissingScope"
	ErrNamespaceNotDefined                        ErrorId = "ErrNamespaceNotDefined"
	ErrNoAppsSpecified                            ErrorId = "ErrNoAppsSpecified"
	ErrNothingTodo                                ErrorId = "ErrNothingTodo"
	ErrRequiredCrdMissing                         ErrorId = "ErrRequiredCrdMissing"
	ErrScaleDownTimeout                           ErrorId = "ErrScaleDownTimeout"
	ErrClusterScaleFlagsConflict                  ErrorId = "ErrClusterScaleFlagsConflict"
	ErrClusterWorkloadWaitTimeout                 ErrorId = "ErrClusterWorkloadWaitTimeout"
	ErrScaleUpTimeout                             ErrorId = "ErrScaleUpTimeout"
	ErrCrdEstablishTimeout                        ErrorId = "ErrCrdEstablishTimeout"
	ErrTemplateNotFound                           ErrorId = "ErrTemplateNotFound"
	ErrValuesCleanupFailed                        ErrorId = "ErrValuesCleanupFailed"
	ErrValuesFailed                               ErrorId = "ErrValuesFailed"
	ErrSyncWindowFailed                           ErrorId = "ErrSyncWindowFailed"
	ErrServerSideApplyFailed                      ErrorId = "ErrServerSideApplyFailed"
	ErrWriteFailed                                ErrorId = "ErrWriteFailed"
	ErrYqFailed                                   ErrorId = "ErrYqFailed"
	ErrChromaHighlightFailed                      ErrorId = "ErrChromaHighlightFailed"
	ErrUninstallDuplicateTemplateResource         ErrorId = "ErrUninstallDuplicateTemplateResource"
	ErrUninstallAmbiguousLeftoverRef              ErrorId = "ErrUninstallAmbiguousLeftoverRef"
	ErrUninstallAmbiguousRefOwnership             ErrorId = "ErrUninstallAmbiguousRefOwnership"
	ErrUninstallRefOwnershipConflictsWithTemplate ErrorId = "ErrUninstallRefOwnershipConflictsWithTemplate"
	// ErrUninstallUnassignedClusterResource is legacy: cluster uninstall no longer aborts on unassigned inventory.
	ErrUninstallUnassignedClusterResource ErrorId = "ErrUninstallUnassignedClusterResource"
)
