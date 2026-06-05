package expect

import "strings"

// HydraDisplayCommand returns the user-visible command (always prefixed with "hydra").
func HydraDisplayCommand(commandPath string, extraArgs ...string) string {
	parts := []string{"hydra"}
	if commandPath != "" {
		parts = append(parts, commandPath)
	}
	parts = append(parts, extraArgs...)
	return strings.Join(parts, " ")
}

// BuildHydraExecLine builds a safely quoted shell invocation for the real binary.
func BuildHydraExecLine(hydraBin, commandPath string, extraArgs ...string) string {
	parts := []string{shellQuote(hydraBin)}
	for _, p := range strings.Fields(commandPath) {
		parts = append(parts, shellQuote(p))
	}
	for _, a := range extraArgs {
		parts = append(parts, shellQuote(a))
	}
	return strings.Join(parts, " ")
}
