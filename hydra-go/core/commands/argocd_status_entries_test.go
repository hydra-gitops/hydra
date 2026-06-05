package commands

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func makeArgocdApplicationForTest(name, project, syncStatus string) unstructured.Unstructured {
	return makeArgocdApplicationForTestWithConditions(name, project, syncStatus, nil)
}

func makeArgocdApplicationForTestWithConditions(name, project, syncStatus string, conditions []map[string]any) unstructured.Unstructured {
	return makeArgocdApplicationForTestWithConditionsAndOp(name, project, syncStatus, conditions, nil)
}

func makeArgocdApplicationForTestWithConditionsAndOp(name, project, syncStatus string, conditions []map[string]any, operationState map[string]any) unstructured.Unstructured {
	status := map[string]any{
		"sync": map[string]any{
			"status": syncStatus,
		},
	}
	if len(conditions) > 0 {
		raw := make([]any, len(conditions))
		for i, c := range conditions {
			raw[i] = c
		}
		status["conditions"] = raw
	}
	if operationState != nil {
		status["operationState"] = operationState
	}
	return unstructured.Unstructured{
		Object: map[string]any{
			"apiVersion": "argoproj.io/v1alpha1",
			"kind":       "Application",
			"metadata": map[string]any{
				"name": name,
			},
			"spec": map[string]any{
				"project": project,
			},
			"status": status,
		},
	}
}

func TestBuildArgocdAppStatusEntries_UsesRealApplicationSyncStatusInsteadOfProjectWindowState(t *testing.T) {
	apps := []unstructured.Unstructured{
		makeArgocdApplicationForTest("prod.apps.api", "prod-apps", "OutOfSync"),
	}

	entries, err := BuildArgocdAppStatusEntries(apps, map[string]syncWindowState{
		"prod-apps": {
			kind:       "allow",
			manualSync: true,
		},
	})
	require.NoError(t, err)
	require.Len(t, entries, 1)

	assert.Equal(t, "prod.apps.api", entries[0].Name)
	assert.Equal(t, "OutOfSync", entries[0].SyncStatus, "the ArgoCD-facing status view must read Application.status.sync.status")
	assert.Equal(t, "auto", entries[0].WindowStatus, "AppProject sync mode stays a separate field and must not overwrite the real Application sync status")
}

func TestBuildArgocdAppStatusEntries_IncludesApplicationStatusConditions(t *testing.T) {
	apps := []unstructured.Unstructured{
		makeArgocdApplicationForTestWithConditions("prod.apps.api", "prod-apps", "Synced", []map[string]any{
			{
				"type":    "ComparisonError",
				"message": "Failed to load target state: failed to get cluster version for cluster \"https://kubernetes.default.svc\": error",
			},
		}),
	}

	entries, err := BuildArgocdAppStatusEntries(apps, map[string]syncWindowState{})
	require.NoError(t, err)
	require.Len(t, entries, 1)

	require.Len(t, entries[0].Conditions, 1)
	assert.Equal(t, "ComparisonError", entries[0].Conditions[0].Type)
	assert.Contains(t, entries[0].Conditions[0].Message, "Failed to load target state")
}

func TestFormatArgocdOperationStateLine_SucceededLastRunDuration(t *testing.T) {
	line, phase := formatArgocdOperationStateLine("Succeeded", "2024-01-01T10:00:00Z", "2024-01-01T10:00:45Z")
	assert.Equal(t, "Succeeded", phase)
	assert.Contains(t, line, "45s")
	assert.Contains(t, line, "Succeeded")
}

func TestFormatArgocdOperationStateLine_RunningElapsed(t *testing.T) {
	started := time.Now().Add(-90 * time.Second).Format(time.RFC3339)
	line, phase := formatArgocdOperationStateLine("Running", started, "")
	assert.Equal(t, "Running", phase)
	assert.Contains(t, line, "elapsed")
	assert.Contains(t, line, "Running")
}

func TestBuildArgocdAppStatusEntries_IncludesOperationState(t *testing.T) {
	apps := []unstructured.Unstructured{
		makeArgocdApplicationForTestWithConditionsAndOp("prod.apps.api", "prod-apps", "Synced", nil, map[string]any{
			"phase":      "Succeeded",
			"startedAt":  "2024-01-01T10:00:00Z",
			"finishedAt": "2024-01-01T10:00:45Z",
		}),
	}

	entries, err := BuildArgocdAppStatusEntries(apps, map[string]syncWindowState{})
	require.NoError(t, err)
	require.Len(t, entries, 1)

	assert.Equal(t, "Succeeded", entries[0].OperationPhase)
	assert.Contains(t, entries[0].OperationLine, "45s")
	assert.Contains(t, entries[0].OperationLine, "sync operation")
}
