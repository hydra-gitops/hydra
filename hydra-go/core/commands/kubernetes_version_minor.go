package commands

import (
	"strconv"
	"strings"
)

// ParseKubernetesMinorFromVersionString returns the minor version from strings such as "1.30",
// "v1.30.0", or "1.30.5-gke.1". Leading digits of the second dot-separated field are used.
// It returns 0 when the string is empty or no minor can be parsed.
func ParseKubernetesMinorFromVersionString(v string) int {
	v = strings.TrimSpace(v)
	if v == "" {
		return 0
	}
	v = strings.TrimPrefix(v, "v")
	parts := strings.Split(v, ".")
	if len(parts) < 2 {
		return 0
	}
	return parseLeadingInt(parts[1])
}

func parseLeadingInt(s string) int {
	s = strings.TrimSpace(s)
	i := 0
	for i < len(s) && s[i] >= '0' && s[i] <= '9' {
		i++
	}
	if i == 0 {
		return 0
	}
	n, err := strconv.Atoi(s[:i])
	if err != nil {
		return 0
	}
	return n
}

// effectiveMinorForLocalBootstrapCatalog returns a minor used to filter RBAC bootstrap names.
// When the Helm/Hydra version string could not be parsed (0), use a high value so the full
// catalog is merged into local template targets.
func effectiveMinorForLocalBootstrapCatalog(parsedMinor int) int {
	if parsedMinor <= 0 {
		return 99
	}
	return parsedMinor
}
