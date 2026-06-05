package entity

import "hydra-gitops.org/hydra/hydra-go/base/log"

// Package LogId
var logIdEntity = log.Hydra().Child("core").Child("entity")

// Struct LogIds
var (
	logIdEntities = logIdEntity.Child("Entities")
)
