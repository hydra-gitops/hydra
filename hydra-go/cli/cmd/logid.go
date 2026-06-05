package cmd

import "hydra-gitops.org/hydra/hydra-go/base/log"

// Package LogId
var logIdCmd = log.Hydra().Child("cli").Child("cmd")
