package helm

import "hydra-gitops.org/hydra/hydra-go/base/log"

// Package LogId
var logIdHelm = log.Hydra().Child("core").Child("helm")

// Struct LogIds
var (
	logIdChartCache     = logIdHelm.Child("ChartCache")
	logIdChartDirectory = logIdHelm.Child("ChartDirectory")
)
