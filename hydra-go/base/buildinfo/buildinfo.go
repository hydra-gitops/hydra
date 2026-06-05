package buildinfo

import "strings"

// Version is the Hydra build version.
//
// It defaults to "dev" for local builds and tests and can be overridden via:
//
//	go build -ldflags "-X hydra-gitops.org/hydra/hydra-go/base/buildinfo.Version=v1.0.0"
var Version = "dev"

// String returns the effective Hydra build version used across the CLI,
// logging, and artifact metadata.
func String() string {
	v := strings.TrimSpace(Version)
	if v == "" {
		return "dev"
	}
	return v
}

// CLIString returns the canonical single-line CLI representation.
func CLIString() string {
	return "hydra " + String()
}
