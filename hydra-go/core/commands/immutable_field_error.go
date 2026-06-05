package commands

import (
	"errors"
	"strings"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
)

// IsImmutableFieldKubernetesError reports whether err indicates that an update
// was rejected because a field cannot be changed (e.g. Secret type). The API
// server typically embeds "immutable" in the message or in StatusDetails causes.
func IsImmutableFieldKubernetesError(err error) bool {
	if err == nil {
		return false
	}

	if strings.Contains(strings.ToLower(err.Error()), "immutable") {
		return true
	}

	var statusErr *apierrors.StatusError
	if !errors.As(err, &statusErr) {
		return false
	}

	st := statusErr.Status()
	if strings.Contains(strings.ToLower(st.Message), "immutable") {
		return true
	}
	if st.Details == nil {
		return false
	}
	for _, c := range st.Details.Causes {
		if strings.Contains(strings.ToLower(c.Message), "immutable") {
			return true
		}
	}
	return false
}
