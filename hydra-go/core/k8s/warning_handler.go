package k8s

import (
	"context"

	"hydra-gitops.org/hydra/hydra-go/base/log"
	"k8s.io/client-go/rest"
)

// Common source labels for [KubernetesAPICtxWarningHandler] (RFC 7234 Warning headers).
const (
	KubernetesAPIWarningSourceServerSideDiff   = "server-side diff"
	KubernetesAPIWarningSourceClusterApplyPlan = "cluster apply (plan)"
	KubernetesAPIWarningSourceClusterScale     = "cluster scale"
)

type warningResourceIDContextKey struct{}

// ContextWithResourceID returns ctx carrying the Kubernetes resource id (e.g. from
// [hydra-gitops.org/hydra/hydra-go/core/types].Id) for the in-flight request. When set,
// [KubernetesAPICtxWarningHandler] includes it in API warning log lines (RFC 7234).
// If unset, the handler logs resource id as "unknown".
func ContextWithResourceID(ctx context.Context, resourceID string) context.Context {
	return context.WithValue(ctx, warningResourceIDContextKey{}, resourceID)
}

func resourceIDFromContext(ctx context.Context) string {
	if v, ok := ctx.Value(warningResourceIDContextKey{}).(string); ok && v != "" {
		return v
	}
	return "unknown"
}

// KubernetesAPICtxWarningHandler implements [rest.WarningHandlerWithContext] by logging
// HTTP Warning headers (typically code 299) with source, code, resource id, and message — unlike
// client-go's default handler, which omits the code.
type KubernetesAPICtxWarningHandler struct {
	Logger log.Logger
	LogID  log.LogId
	// Source identifies the operation (e.g. [KubernetesAPIWarningSourceServerSideDiff]).
	Source string
	// Debug when true logs at DEBUG; otherwise INFO.
	Debug bool
}

var _ rest.WarningHandlerWithContext = KubernetesAPICtxWarningHandler{}

func (h KubernetesAPICtxWarningHandler) HandleWarningHeaderWithContext(ctx context.Context, code int, _ string, text string) {
	if code != 299 || len(text) == 0 {
		return
	}
	src := h.Source
	if src == "" {
		src = "Kubernetes API"
	}
	msg := "{source} status code {code} for id {resourceId}: {warning}"
	args := []any{
		log.String("source", src),
		log.Int("code", code),
		log.String("resourceId", resourceIDFromContext(ctx)),
		log.String("warning", text),
	}
	if h.Debug {
		h.Logger.DebugLog(h.LogID, msg, args...)
	} else {
		h.Logger.Info(h.LogID, msg, args...)
	}
}
