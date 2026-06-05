package commands

import (
	"strings"

	"hydra-gitops.org/hydra/hydra-go/core/entity"
	"hydra-gitops.org/hydra/hydra-go/core/types"
)

const argoCdSyncOptionsAnnotation = "argocd.argoproj.io/sync-options"

func ShouldServerSideApply(e entity.Entity, key types.EntityKeyUnstructured) bool {
	u, err := e.UnstructuredOrError(key)
	if err != nil {
		return false
	}

	annotations := u.GetAnnotations()
	if annotations == nil {
		return false
	}

	syncOptions, ok := annotations[argoCdSyncOptionsAnnotation]
	if !ok {
		return false
	}

	for _, opt := range strings.Split(syncOptions, ",") {
		if strings.TrimSpace(opt) == "ServerSideApply=true" {
			return true
		}
	}

	return false
}
