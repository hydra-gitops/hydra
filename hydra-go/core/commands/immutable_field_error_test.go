package commands

import (
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestIsImmutableFieldKubernetesError_MessageContainsImmutable(t *testing.T) {
	err := errors.New(`Secret "pairing-cert" is invalid: type: Invalid value: "Opaque": field is immutable`)
	assert.True(t, IsImmutableFieldKubernetesError(err))
}

func TestIsImmutableFieldKubernetesError_StatusErrorMessage(t *testing.T) {
	err := &apierrors.StatusError{ErrStatus: metav1.Status{
		Status:  metav1.StatusFailure,
		Message: `Secret "x" is invalid: type: field is immutable`,
		Reason:  metav1.StatusReasonInvalid,
	}}
	assert.True(t, IsImmutableFieldKubernetesError(err))
}

func TestIsImmutableFieldKubernetesError_StatusDetailsCauses(t *testing.T) {
	err := &apierrors.StatusError{ErrStatus: metav1.Status{
		Status: metav1.StatusFailure,
		Reason: metav1.StatusReasonInvalid,
		Details: &metav1.StatusDetails{
			Causes: []metav1.StatusCause{
				{
					Type:    metav1.CauseTypeFieldValueInvalid,
					Message: `Invalid value: "Opaque": field is immutable`,
					Field:   "type",
				},
			},
		},
	}}
	assert.True(t, IsImmutableFieldKubernetesError(err))
}

func TestIsImmutableFieldKubernetesError_Wrapped(t *testing.T) {
	inner := &apierrors.StatusError{ErrStatus: metav1.Status{
		Status:  metav1.StatusFailure,
		Message: `Secret "x" is invalid: spec: field is immutable`,
	}}
	err := fmt.Errorf("patch failed: %w", inner)
	assert.True(t, IsImmutableFieldKubernetesError(err))
}

func TestIsImmutableFieldKubernetesError_OtherInvalidError(t *testing.T) {
	err := &apierrors.StatusError{ErrStatus: metav1.Status{
		Status:  metav1.StatusFailure,
		Message: `ConfigMap "x" is invalid: data.foo: Required value`,
		Reason:  metav1.StatusReasonInvalid,
	}}
	assert.False(t, IsImmutableFieldKubernetesError(err))
}

func TestIsImmutableFieldKubernetesError_Nil(t *testing.T) {
	assert.False(t, IsImmutableFieldKubernetesError(nil))
}
