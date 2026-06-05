package references

import "hydra-gitops.org/hydra/hydra-go/base/log"

// Package LogId
var logIdReferences = log.Hydra().Child("core").Child("references")
