package cmd

import (
	"strings"
	"testing"

	"hydra-gitops.org/hydra/hydra-go/base/record"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
)

func TestCollectHelpCommandPaths_MatchesProductionCLI(t *testing.T) {
	mock := newMockRootCommand()
	rootCmd, _ := newRootCommand(mock.rootCommandParams())

	got := record.CollectHelpCommandPaths(rootCmd)
	gotPaths := make([]string, 0, len(got))
	for _, c := range got {
		gotPaths = append(gotPaths, c.Path)
	}

	required := []string{
		"local",
		"local template",
		"cluster",
		"gitops",
		"gitops uninstall",
	}
	for _, want := range required {
		assert.Contains(t, gotPaths, want)
	}

	// Independent reference: same walk rules as base/record/commands_test.go
	wantPaths := referenceProductionHelpPaths(rootCmd)
	assert.ElementsMatch(t, wantPaths, gotPaths)
	assert.Greater(t, len(gotPaths), 20, "expected a substantial command tree")
}

func referenceProductionHelpPaths(root *cobra.Command) []string {
	var out []string
	var walk func(*cobra.Command, []string)
	walk = func(cmd *cobra.Command, prefix []string) {
		for _, child := range cmd.Commands() {
			if child.Hidden {
				continue
			}
			parts := splitCommandUse(child.Use)
			if len(parts) == 0 {
				continue
			}
			name := parts[0]
			path := append(append([]string{}, prefix...), name)
			if len(path) > 0 && path[0] == "record" {
				continue
			}
			out = append(out, strings.Join(path, " "))
			walk(child, path)
		}
	}
	walk(root, nil)
	return out
}

func splitCommandUse(use string) []string {
	name := strings.Fields(use)
	if len(name) == 0 {
		return nil
	}
	return []string{name[0]}
}

func TestRootCommandContainsRecord(t *testing.T) {
	mock := newMockRootCommand()
	rootCmd, _ := newRootCommand(mock.rootCommandParams())
	assert.Contains(t, commandUseNames(rootCmd.Commands()), "record")
}
