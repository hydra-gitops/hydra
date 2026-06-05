package commands

import (
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- ResolvePatterns tests ---

func TestResolvePatterns_ExactNameReturnedAsIs(t *testing.T) {
	result, warnings, err := ResolvePatterns(
		[]string{"in-cluster.cluster-infra.kyverno"},
		[]string{"in-cluster.cluster-infra.kyverno", "in-cluster.cluster-infra.cert-manager"},
	)
	require.NoError(t, err)
	assert.Equal(t, []string{"in-cluster.cluster-infra.kyverno"}, result)
	assert.Empty(t, warnings)
}

func TestResolvePatterns_ExactNameNotValidatedAgainstAppList(t *testing.T) {
	result, _, err := ResolvePatterns(
		[]string{"nonexistent.app"},
		[]string{"prod.a", "prod.b"},
	)
	require.NoError(t, err)
	assert.Equal(t, []string{"nonexistent.app"}, result)
}

func TestResolvePatterns_StarDoesNotMatchDot(t *testing.T) {
	result, _, err := ResolvePatterns(
		[]string{"prod.*"},
		[]string{"prod.a", "prod.b", "prod.a.child"},
	)
	require.NoError(t, err)
	assert.Equal(t, []string{"prod.a", "prod.b"}, result)
}

func TestResolvePatterns_DoubleStarMatchesDots(t *testing.T) {
	result, _, err := ResolvePatterns(
		[]string{"prod.**"},
		[]string{"prod.a", "prod.a.child", "test.c"},
	)
	require.NoError(t, err)
	assert.Equal(t, []string{"prod.a", "prod.a.child"}, result)
}

func TestResolvePatterns_WildcardNoMatchReturnsError(t *testing.T) {
	_, _, err := ResolvePatterns(
		[]string{"foo*"},
		[]string{"prod.a", "prod.b"},
	)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "foo*")
}

func TestResolvePatterns_MultiplePatternsDeduplicatedAndSorted(t *testing.T) {
	result, _, err := ResolvePatterns(
		[]string{"prod.a", "prod.*"},
		[]string{"prod.a", "prod.b"},
	)
	require.NoError(t, err)
	assert.Equal(t, []string{"prod.a", "prod.b"}, result)
}

func TestResolvePatterns_EmptyPatterns(t *testing.T) {
	result, _, err := ResolvePatterns(
		[]string{},
		[]string{"prod.a", "prod.b"},
	)
	require.NoError(t, err)
	assert.Empty(t, result)
}

func TestResolvePatterns_DoubleStarMatchesAll(t *testing.T) {
	result, _, err := ResolvePatterns(
		[]string{"**"},
		[]string{"prod.a", "test.b", "dev.c"},
	)
	require.NoError(t, err)
	assert.Equal(t, []string{"dev.c", "prod.a", "test.b"}, result)
}

func TestResolvePatterns_MultiplePatterns_OneFailsReturnsError(t *testing.T) {
	_, _, err := ResolvePatterns(
		[]string{"prod.*", "foo*"},
		[]string{"prod.a"},
	)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "foo*")
}

func TestResolvePatterns_GlobPatternMatchesByGlob(t *testing.T) {
	result, _, err := ResolvePatterns(
		[]string{"prod*bar"},
		[]string{"prod.a", "prodbar", "prod*bar"},
	)
	require.NoError(t, err)
	assert.Equal(t, []string{"prod*bar", "prodbar"}, result)
}

func TestResolvePatterns_DoubleStarWithEmptyAppList(t *testing.T) {
	_, _, err := ResolvePatterns(
		[]string{"**"},
		[]string{},
	)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "**")
}

func TestResolvePatterns_TripleSegmentWildcardMatchesChildApps(t *testing.T) {
	result, _, err := ResolvePatterns(
		[]string{"*.*.*"},
		[]string{"prod.a", "prod.a.child1", "prod.a.child2"},
	)
	require.NoError(t, err)
	assert.Equal(t, []string{"prod.a.child1", "prod.a.child2"}, result)
}

func TestResolvePatterns_WarningForSingleMatch(t *testing.T) {
	result, warnings, err := ResolvePatterns(
		[]string{"prod.infra.*"},
		[]string{"prod.infra.monitoring", "prod.demo.app1", "prod.demo.app2"},
	)
	require.NoError(t, err)
	assert.Equal(t, []string{"prod.infra.monitoring"}, result)
	require.Len(t, warnings, 1)
	assert.Contains(t, warnings[0], "prod.infra.*")
	assert.Contains(t, warnings[0], "only 1 application")
}

func TestResolvePatterns_NoWarningForMultipleMatches(t *testing.T) {
	_, warnings, err := ResolvePatterns(
		[]string{"prod.*"},
		[]string{"prod.a", "prod.b"},
	)
	require.NoError(t, err)
	assert.Empty(t, warnings)
}

func TestArgocdStatusTableRowWidth_MatchesPrintedRow(t *testing.T) {
	w := argocdStatusTableRowWidth(10, 6, 4, 15)
	row := argocdTableRowPrefix +
		padRightASCII("aaaaaaaaaa", 10) + argocdTableColGap +
		padRightASCII("Synced", 6) + argocdTableColGap +
		padRightASCII("auto", 4) + argocdTableColGap +
		padRightASCII("none", 15)
	assert.Equal(t, len(row), w)
}

func TestNewArgocdStatusTableWidths_UsesGlobalMaximaAcrossEntries(t *testing.T) {
	widths := newArgocdStatusTableWidths([]ArgocdAppStatusEntry{
		{
			Name:          "short.app",
			SyncStatus:    "Synced",
			WindowStatus:  "auto",
			OperationLine: "sync operation: Succeeded (last run 0s)",
		},
		{
			Name:          "very.long.application.name",
			SyncStatus:    "Unknown",
			WindowStatus:  "prevent",
			OperationLine: "sync operation: Running (123s elapsed)",
		},
	})

	assert.Equal(t, len("very.long.application.name"), widths.Name)
	assert.Equal(t, len("Unknown"), widths.State)
	assert.Equal(t, len("prevent"), widths.Mode)
	assert.Equal(t, len("Succeeded (last run 0s)"), widths.Op)
	assert.Equal(t, argocdStatusTableRowWidth(widths.Name, widths.State, widths.Mode, widths.Op), widths.Row)
}

func TestOperationCellText_StripsSyncOperationPrefix(t *testing.T) {
	assert.Equal(t, "", operationCellText(""))
	assert.Equal(t, "Succeeded (last run 12s)", operationCellText("sync operation: Succeeded (last run 12s)"))
}

func TestWrapWords_BreaksAtWordBoundaries(t *testing.T) {
	long := "ComparisonError: Failed to load target state: failed to generate manifest for source 1 of 1: rpc error: code = Unknown desc = Manifest generation error (cached): charts-repository/apps/demo/service-api-usage/dev: app path does not exist"
	lines := wrapWords(long, 40)
	require.GreaterOrEqual(t, len(lines), 2)
	for _, ln := range lines {
		assert.LessOrEqual(t, utf8.RuneCountInString(ln), 40)
	}
	assert.Contains(t, strings.Join(lines, " "), "does not exist")
}

func TestWrapWords_NormalizesEmbeddedNewlines(t *testing.T) {
	lines := wrapWords("hello\nworld", 80)
	require.Len(t, lines, 1)
	assert.Equal(t, "hello world", lines[0])
}

func TestBreakNewlineAfterColonSpacePastWidth_InsertsBreakAtColon(t *testing.T) {
	const blockW = 40
	// Colon at rune index blockW; ": " should break.
	prefix := strings.Repeat("a", blockW)
	s := prefix + ": tail message"
	out := breakNewlineAfterColonSpacePastWidth(s, blockW)
	assert.Contains(t, out, "\n")
	assert.True(t, strings.HasPrefix(out, prefix+":"))
	assert.Contains(t, out, "tail message")
}

func TestBreakNewlineAfterColonSpacePastWidth_DoesNotBreakEarlyColon(t *testing.T) {
	s := "ComparisonError: short"
	out := breakNewlineAfterColonSpacePastWidth(s, 40)
	assert.Equal(t, s, out)
}

func TestBreakOneLineColonAfterWidth_RecursesOnlyOnRight(t *testing.T) {
	const w = 10
	// First ": " at index 12; only the right side is processed again (another ": " there).
	s := "0123456789ab: " + strings.Repeat("c", 25) + ": end"
	parts := breakOneLineColonAfterWidth(s, w)
	require.GreaterOrEqual(t, len(parts), 3)
	assert.Equal(t, "0123456789ab:", parts[0])
	assert.Equal(t, "end", parts[len(parts)-1])
}

func TestBreakOneLineColonAfterWidth_LongLeftUnchangedUntilFirstSplit(t *testing.T) {
	const w = 10
	// Long prefix before first ": " at/after w — left line is emitted whole (not split again on the left).
	s := strings.Repeat("a", 11) + ": tail"
	parts := breakOneLineColonAfterWidth(s, w)
	require.Len(t, parts, 2)
	assert.Equal(t, strings.Repeat("a", 11)+":", parts[0])
	assert.Equal(t, "tail", parts[1])
}

func TestSplitConditionDisplayLines_DoesNotWrapLeftSideAfterColonSplit(t *testing.T) {
	const w = 10
	s := strings.Repeat("a", 11) + ": " + strings.Repeat("b", 11) + ": end"
	lines := splitConditionDisplayLines(s, w)
	require.GreaterOrEqual(t, len(lines), 3)
	assert.Equal(t, strings.Repeat("a", 11)+":", lines[0])
	assert.Equal(t, strings.Repeat("b", 11)+":", lines[1])
	assert.Equal(t, "end", lines[len(lines)-1])
}
