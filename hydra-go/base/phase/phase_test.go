package phase

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type testState struct {
	calls []string
}

func TestBuilderAssignsNumbersInOrder(t *testing.T) {
	phases := NewBuilder[testState]().
		Add("crds", "applying CRDs", func(context.Context, *testState) Result {
			return Next()
		}).
		Add("namespaces", "applying namespaces", func(context.Context, *testState) Result {
			return Next()
		}).
		Add("restore", "restoring backup secrets", func(context.Context, *testState) Result {
			return Next()
		}).
		Build()

	require.Len(t, phases.Items, 3)
	assert.Equal(t, 1, phases.Items[0].Number)
	assert.Equal(t, 2, phases.Items[1].Number)
	assert.Equal(t, 3, phases.Items[2].Number)
	assert.Equal(t, "crds", phases.Items[0].Name)
	assert.Equal(t, "restore", phases.Items[2].Name)
}

func TestPhasesRun_StopsOnAborted(t *testing.T) {
	state := &testState{}
	stopErr := errors.New("stop here")
	var reports []Report

	err := NewBuilder[testState]().
		Add("phase-a", "phase A", func(context.Context, *testState) Result {
			state.calls = append(state.calls, "phase-a")
			return Next()
		}).
		Add("phase-b", "phase B", func(context.Context, *testState) Result {
			state.calls = append(state.calls, "phase-b")
			return Skipped("nothing to do")
		}).
		Add("phase-c", "phase C", func(context.Context, *testState) Result {
			state.calls = append(state.calls, "phase-c")
			return Aborted(stopErr)
		}).
		Add("phase-d", "phase D", func(context.Context, *testState) Result {
			t.Fatal("phase-d must not run after aborted")
			return Next()
		}).
		Build().
		Run(context.Background(), state, func(report Report) {
			reports = append(reports, report)
		})

	require.ErrorIs(t, err, stopErr)
	assert.Equal(t, []string{"phase-a", "phase-b", "phase-c"}, state.calls)
	require.Len(t, reports, 3)
	assert.Equal(t, StatusNext, reports[0].Status)
	assert.Equal(t, StatusSkipped, reports[1].Status)
	assert.Equal(t, StatusAborted, reports[2].Status)
}

func TestReportMessageFormatsStatuses(t *testing.T) {
	nextReport := Report{
		Number:      1,
		Total:       10,
		Description: "applying CRDs",
		Status:      StatusNext,
	}
	skippedReport := Report{
		Number:      3,
		Total:       10,
		Description: "restoring backup secrets",
		Status:      StatusSkipped,
	}
	abortedReport := Report{
		Number:      6,
		Total:       10,
		Description: "scaling up workloads",
		Status:      StatusAborted,
	}

	assert.Equal(t, "phase 1/10: applying CRDs", nextReport.Message())
	assert.Equal(t, "phase 3/10: restoring backup secrets (skipped)", skippedReport.Message())
	assert.Equal(t, "phase 6/10: scaling up workloads (aborted)", abortedReport.Message())
}
