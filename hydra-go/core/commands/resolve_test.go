package commands

import (
	"testing"

	"hydra-gitops.org/hydra/hydra-go/base/log"
	"hydra-gitops.org/hydra/hydra-go/core/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/util/sets"
)

func TestResolveAppIdsFromConfig_ExactAppIdWithoutWildcard(t *testing.T) {
	allApps := []string{"prod.demo.app1"}
	result, err := resolveAppIdsExactWithValidation(
		[]types.AppIdPattern{"prod.demo.app1"},
		allApps,
	)
	require.NoError(t, err)
	assert.True(t, result.Has(types.AppId("prod.demo.app1")))
	assert.Equal(t, 1, result.Len())
}

func TestResolveAppIdsFromConfig_MultipleExactAppIds(t *testing.T) {
	allApps := []string{"prod.demo.app1", "prod.demo.app2"}
	result, err := resolveAppIdsExactWithValidation(
		[]types.AppIdPattern{"prod.demo.app1", "prod.demo.app2"},
		allApps,
	)
	require.NoError(t, err)
	assert.True(t, result.Has(types.AppId("prod.demo.app1")))
	assert.True(t, result.Has(types.AppId("prod.demo.app2")))
	assert.Equal(t, 2, result.Len())
}

func TestResolveAppIdsFromConfig_InvalidAppIdWithoutWildcard(t *testing.T) {
	_, err := resolveAppIdsExactWithValidation(
		[]types.AppIdPattern{"invalidformat"},
		nil,
	)
	require.Error(t, err)
}

func TestResolveAppIdsFromConfig_EmptyPatterns(t *testing.T) {
	result, err := resolveAppIdsExactWithValidation(
		[]types.AppIdPattern{},
		nil,
	)
	require.NoError(t, err)
	assert.Equal(t, 0, result.Len())
}

func TestResolveAppIdsFromConfig_ExactAppIdNotInRepository(t *testing.T) {
	allApps := []string{"prod.demo.app1"}
	_, err := resolveAppIdsExactWithValidation(
		[]types.AppIdPattern{"prod.demo.ghost"},
		allApps,
	)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "prod.demo.ghost")
}

func TestResolveAppIdsFromConfig_MixedGlobWithUnknownExact(t *testing.T) {
	allApps := []string{"prod.demo.app1"}
	_, err := resolveAppIdsWithAllApps(
		[]types.AppIdPattern{"prod.demo.*", "prod.unknown.app"},
		nil,
		allApps,
	)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "prod.unknown.app")
}

func TestResolveAppIdsFromConfig_WildcardMatchesRootAppsOnly(t *testing.T) {
	allApps := []string{
		"prod.demo", "prod.infra", "prod.demo.app1",
		"dev.demo",
	}
	result, err := resolveAppIdsWithAllApps(
		[]types.AppIdPattern{"prod.*"},
		nil,
		allApps,
	)
	require.NoError(t, err)
	assert.True(t, result.Has(types.AppId("prod.demo")))
	assert.True(t, result.Has(types.AppId("prod.infra")))
	assert.False(t, result.Has(types.AppId("prod.demo.app1")))
	assert.Equal(t, 2, result.Len())
}

func TestResolveAppIdsFromConfig_DoubleStarMatchesAll(t *testing.T) {
	allApps := []string{
		"prod.demo", "prod.infra", "prod.demo.app1",
		"dev.demo",
	}
	result, err := resolveAppIdsWithAllApps(
		[]types.AppIdPattern{"prod.**"},
		nil,
		allApps,
	)
	require.NoError(t, err)
	assert.True(t, result.Has(types.AppId("prod.demo")))
	assert.True(t, result.Has(types.AppId("prod.infra")))
	assert.True(t, result.Has(types.AppId("prod.demo.app1")))
	assert.Equal(t, 3, result.Len())
}

func TestResolveAppIdsFromConfig_WildcardMatchesChildApps(t *testing.T) {
	allApps := []string{
		"prod.demo.app1", "prod.demo.app2", "prod.infra.monitoring",
	}
	result, err := resolveAppIdsWithAllApps(
		[]types.AppIdPattern{"prod.demo.*"},
		nil,
		allApps,
	)
	require.NoError(t, err)
	assert.True(t, result.Has(types.AppId("prod.demo.app1")))
	assert.True(t, result.Has(types.AppId("prod.demo.app2")))
	assert.Equal(t, 2, result.Len())
}

func TestResolveAppIdsFromConfig_WildcardNoMatch(t *testing.T) {
	allApps := []string{
		"prod.demo.app1", "prod.demo.app2",
	}
	_, err := resolveAppIdsWithAllApps(
		[]types.AppIdPattern{"staging.*"},
		nil,
		allApps,
	)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "staging.*")
}

func TestResolveAppIdsFromConfig_DeduplicationWithOverlappingPatterns(t *testing.T) {
	allApps := []string{
		"prod.demo.app1", "prod.demo.app2", "prod.infra.monitoring",
	}
	result, err := resolveAppIdsWithAllApps(
		[]types.AppIdPattern{"prod.*.*", "prod.demo.*"},
		nil,
		allApps,
	)
	require.NoError(t, err)
	assert.Equal(t, 3, result.Len())
}

// --- Exclude tests ---

func TestResolveAppIdsFromConfig_ExcludeExactApp(t *testing.T) {
	allApps := []string{
		"prod.demo.app1", "prod.demo.app2", "prod.infra.monitoring",
	}
	result, err := resolveAppIdsWithAllApps(
		[]types.AppIdPattern{"prod.*.*"},
		[]types.AppIdPattern{"prod.demo.app1"},
		allApps,
	)
	require.NoError(t, err)
	assert.False(t, result.Has(types.AppId("prod.demo.app1")))
	assert.True(t, result.Has(types.AppId("prod.demo.app2")))
	assert.True(t, result.Has(types.AppId("prod.infra.monitoring")))
	assert.Equal(t, 2, result.Len())
}

func TestResolveAppIdsFromConfig_ExcludeGlobPattern(t *testing.T) {
	allApps := []string{
		"prod.demo.app1", "prod.demo.app2", "prod.infra.monitoring",
	}
	result, err := resolveAppIdsWithAllApps(
		[]types.AppIdPattern{"prod.*.*"},
		[]types.AppIdPattern{"prod.demo.*"},
		allApps,
	)
	require.NoError(t, err)
	assert.False(t, result.Has(types.AppId("prod.demo.app1")))
	assert.False(t, result.Has(types.AppId("prod.demo.app2")))
	assert.True(t, result.Has(types.AppId("prod.infra.monitoring")))
	assert.Equal(t, 1, result.Len())
}

func TestResolveAppIdsFromConfig_ExcludeWithNoMatchIsHarmless(t *testing.T) {
	allApps := []string{
		"prod.demo.app1", "prod.demo.app2",
	}
	result, err := resolveAppIdsWithAllApps(
		[]types.AppIdPattern{"prod.demo.*"},
		[]types.AppIdPattern{"prod.nonexistent.app"},
		allApps,
	)
	require.NoError(t, err)
	assert.Equal(t, 2, result.Len())
}

func TestResolveAppIdsFromConfig_ExcludeMultiplePatterns(t *testing.T) {
	allApps := []string{
		"prod.demo.app1", "prod.demo.app2", "prod.infra.monitoring", "prod.infra.logging",
	}
	result, err := resolveAppIdsWithAllApps(
		[]types.AppIdPattern{"prod.*.*"},
		[]types.AppIdPattern{"prod.demo.app1", "prod.infra.logging"},
		allApps,
	)
	require.NoError(t, err)
	assert.True(t, result.Has(types.AppId("prod.demo.app2")))
	assert.True(t, result.Has(types.AppId("prod.infra.monitoring")))
	assert.Equal(t, 2, result.Len())
}

func TestResolveAppIdsFromConfig_ExcludeAllResultsInEmptySet(t *testing.T) {
	allApps := []string{
		"prod.demo.app1", "prod.demo.app2",
	}
	result, err := resolveAppIdsWithAllApps(
		[]types.AppIdPattern{"prod.demo.*"},
		[]types.AppIdPattern{"prod.demo.*"},
		allApps,
	)
	require.NoError(t, err)
	assert.Equal(t, 0, result.Len())
}

func TestResolveAppIdsFromConfig_ExcludeExactWithoutWildcardInIncludes(t *testing.T) {
	allApps := []string{
		"prod.demo.app1", "prod.demo.app2",
	}
	result, err := resolveAppIdsWithAllApps(
		[]types.AppIdPattern{"prod.demo.app1", "prod.demo.app2"},
		[]types.AppIdPattern{"prod.demo.app1"},
		allApps,
	)
	require.NoError(t, err)
	assert.False(t, result.Has(types.AppId("prod.demo.app1")))
	assert.True(t, result.Has(types.AppId("prod.demo.app2")))
	assert.Equal(t, 1, result.Len())
}

// --- helpers ---

func resolveAppIdsExactWithValidation(patterns []types.AppIdPattern, allAppNames []string) (sets.Set[types.AppId], error) {
	if len(patterns) == 0 {
		return sets.New[types.AppId](), nil
	}

	result := sets.New[types.AppId]()
	for _, p := range patterns {
		appId, err := types.NewAppId(string(p))
		if err != nil {
			return nil, err
		}
		result.Insert(appId)
	}
	return result, validateAppIdsAgainstEnumerated(result, allAppNames)
}

func resolveAppIdsWithAllApps(patterns []types.AppIdPattern, excludePatterns []types.AppIdPattern, allAppNames []string) (sets.Set[types.AppId], error) {
	raw := make([]string, len(patterns))
	for i, p := range patterns {
		raw[i] = string(p)
	}
	resolved, _, err := ResolvePatterns(raw, allAppNames)
	if err != nil {
		return nil, err
	}

	result := sets.New[types.AppId]()
	for _, name := range resolved {
		result.Insert(types.AppId(name))
	}

	if len(excludePatterns) > 0 {
		rawExclude := make([]string, len(excludePatterns))
		for i, p := range excludePatterns {
			rawExclude[i] = string(p)
		}
		result, err = applyExcludes(log.Default(), result, rawExclude, allAppNames)
		if err != nil {
			return nil, err
		}
	}

	if err := validateAppIdsAgainstEnumerated(result, allAppNames); err != nil {
		return nil, err
	}

	return result, nil
}
