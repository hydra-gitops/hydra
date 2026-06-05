package types

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMatchAppIdGlob_StarDoesNotMatchDot(t *testing.T) {
	assert.False(t, MatchAppIdGlob("prod.*", "prod.demo.app1"))
}

func TestMatchAppIdGlob_StarMatchesWithinSegment(t *testing.T) {
	assert.True(t, MatchAppIdGlob("prod.*", "prod.demo"))
}

func TestMatchAppIdGlob_DoubleStarMatchesAcrossDots(t *testing.T) {
	assert.True(t, MatchAppIdGlob("prod.**", "prod.demo.app1"))
}

func TestMatchAppIdGlob_TripleSegmentWildcard(t *testing.T) {
	assert.True(t, MatchAppIdGlob("*.*.*", "prod.infra.nginx"))
	assert.False(t, MatchAppIdGlob("*.*.*", "prod.infra"))
}

func TestMatchAppIdGlob_TrailingStarWithoutDot(t *testing.T) {
	assert.True(t, MatchAppIdGlob("prod.cluster-infra*", "prod.cluster-infra"))
}

func TestMatchAppIdGlob_TrailingStarDoesNotCrossDot(t *testing.T) {
	assert.False(t, MatchAppIdGlob("prod.cluster-infra*", "prod.cluster-infra.nginx"))
}

func TestMatchAppIdGlob_DoubleStarAloneMatchesEverything(t *testing.T) {
	assert.True(t, MatchAppIdGlob("**", "prod.infra.nginx"))
	assert.True(t, MatchAppIdGlob("**", "a"))
	assert.True(t, MatchAppIdGlob("**", "a.b"))
}

func TestMatchAppIdGlob_ExactMatchWithoutWildcard(t *testing.T) {
	assert.True(t, MatchAppIdGlob("prod.infra", "prod.infra"))
}

func TestMatchAppIdGlob_ExactMatchMismatch(t *testing.T) {
	assert.False(t, MatchAppIdGlob("prod.infra", "prod.demo"))
}

func TestMatchAppIdGlob_MidSegmentWildcard(t *testing.T) {
	assert.True(t, MatchAppIdGlob("prod.cluster-*.*", "prod.cluster-infra.nginx"))
	assert.False(t, MatchAppIdGlob("prod.cluster-*.*", "prod.cluster-infra"))
}

func TestMatchAppIdGlob_DoubleStarMidPattern(t *testing.T) {
	assert.True(t, MatchAppIdGlob("prod.**.nginx", "prod.cluster-infra.nginx"))
	assert.True(t, MatchAppIdGlob("prod.**.nginx", "prod.demo.nginx"))
	assert.False(t, MatchAppIdGlob("prod.**.nginx", "dev.cluster-infra.nginx"))
}

func TestMatchAppIdGlob_TwoSegmentWildcard(t *testing.T) {
	assert.True(t, MatchAppIdGlob("*.*", "prod.demo"))
	assert.False(t, MatchAppIdGlob("*.*", "prod.demo.app1"))
}

func TestMatchAppIdGlob_SingleStarAlone(t *testing.T) {
	assert.True(t, MatchAppIdGlob("*", "prod"))
	assert.False(t, MatchAppIdGlob("*", "prod.demo"))
}

func TestIsGlobPattern_WithStar(t *testing.T) {
	assert.True(t, IsGlobPattern("prod.*"))
	assert.True(t, IsGlobPattern("**"))
	assert.True(t, IsGlobPattern("*.*.*"))
}

func TestIsGlobPattern_WithoutStar(t *testing.T) {
	assert.False(t, IsGlobPattern("prod.demo"))
	assert.False(t, IsGlobPattern("prod.demo.app1"))
}
