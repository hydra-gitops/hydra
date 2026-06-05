package ci

import (
	"fmt"
	"strings"
)

// ParseChartPath parses a relative chart directory path of the form
// "<rootAppsPath>/<group>/<app>/<env>" and returns the last three segments.
func ParseChartPath(path string) (group, app, env string, err error) {
	parts := strings.Split(path, "/")
	if len(parts) < 4 {
		return "", "", "", fmt.Errorf("invalid chart path: %s (expected at least 4 segments)", path)
	}
	n := len(parts)
	return parts[n-3], parts[n-2], parts[n-1], nil
}
