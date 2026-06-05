package action

import "hydra-gitops.org/hydra/hydra-go/base/log"

// Package LogId
var logIdAction = log.Hydra().Child("cli").Child("action")

// logIdApplyDryRunDiff classifies each existing entity after SSA dry-run (cluster vs dry-run YAML).
var logIdApplyDryRunDiff = logIdAction.Child("apply-dry-run-diff")
