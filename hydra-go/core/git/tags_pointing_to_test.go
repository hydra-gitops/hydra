package git

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestTagsPointingTo_MultipleTagsSameCommit(t *testing.T) {
	dir := t.TempDir()
	repo := Init(dir).
		Commit("init", "a.txt", "hello").
		Tag("build-202601011200").
		Tag("demo-service-ui-1.0.0-dev")
	require.NoError(t, repo.Err)

	tags, err := repo.TagsPointingTo("HEAD")
	require.NoError(t, err)
	require.ElementsMatch(t, []string{"build-202601011200", "demo-service-ui-1.0.0-dev"}, tags)
}

func TestTagsPointingTo_OnlyTagsOnHeadCommit(t *testing.T) {
	dir := t.TempDir()
	repo := Init(dir).
		Commit("init", "a.txt", "hello").
		Tag("build-old").
		Commit("second", "a.txt", "world").
		Tag("build-new")
	require.NoError(t, repo.Err)

	tags, err := repo.TagsPointingTo("HEAD")
	require.NoError(t, err)
	require.Equal(t, []string{"build-new"}, tags)
}

func TestTagsPointingTo_OpenRepoBySubdir(t *testing.T) {
	dir := t.TempDir()
	apps := filepath.Join(dir, "apps")
	r := Init(dir).
		Commit("init", "apps/chart/Chart.yaml", "x").
		Tag("v1")
	require.NoError(t, r.Err)

	repo := Open(apps)
	require.NoError(t, repo.Err)
	tags, err := repo.TagsPointingTo("HEAD")
	require.NoError(t, err)
	require.Equal(t, []string{"v1"}, tags)
}
