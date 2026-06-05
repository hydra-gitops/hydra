package cel

import (
	"strings"
)

// ObjectsetRioOwnerGVKToHydraGvk converts a Rancher wrangler
// objectset.rio.cattle.io/owner-gvk annotation value into Hydra's GVK string
// form (version/kind for the core API group, otherwise group/version/kind).
//
// Supported examples:
//
//	"/v1, Kind=Service"                     -> "v1/Service"
//	"apps/v1, Kind=Deployment"             -> "apps/v1/Deployment"
//	"networking.k8s.io/v1, Kind=Ingress"   -> "networking.k8s.io/v1/Ingress"
//
// Malformed or empty input returns "".
func ObjectsetRioOwnerGVKToHydraGvk(raw string) string {
	s := strings.TrimSpace(raw)
	if s == "" {
		return ""
	}

	lower := strings.ToLower(s)
	sep := ", kind="
	idx := strings.Index(lower, sep)
	if idx < 0 {
		return ""
	}

	gvPart := strings.TrimSpace(s[:idx])
	kindPart := strings.TrimSpace(s[idx+len(sep):])
	if gvPart == "" || kindPart == "" {
		return ""
	}

	gvPart = strings.TrimPrefix(gvPart, "/")
	gvPart = strings.TrimSpace(gvPart)
	if gvPart == "" {
		return ""
	}

	switch strings.Count(gvPart, "/") {
	case 0:
		// Core group: apiVersion only (e.g. "v1").
		return gvPart + "/" + kindPart
	case 1:
		// group/version (e.g. "apps/v1").
		return gvPart + "/" + kindPart
	default:
		return ""
	}
}
