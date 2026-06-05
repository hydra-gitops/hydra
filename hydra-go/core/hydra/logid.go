package hydra

import "hydra-gitops.org/hydra/hydra-go/base/log"

// Package LogId
var logIdHydra = log.Hydra().Child("core").Child("hydra")

// Struct LogIds
var (
	logIdContext  = logIdHydra.Child("Context")
	logIdCluster  = logIdHydra.Child("Cluster")
	logIdRootApp  = logIdHydra.Child("RootApp")
	logIdChildApp = logIdHydra.Child("ChildApp")
)
