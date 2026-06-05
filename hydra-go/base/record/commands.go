package record

import (
	"strings"

	"github.com/spf13/cobra"
)

// HelpCommand describes one Hydra CLI command whose help text should be recorded.
type HelpCommand struct {
	// Path is the command path without the "hydra" prefix (e.g. "gitops uninstall").
	Path string
	// Slug is a filesystem-safe name derived from Path (spaces become hyphens).
	Slug string
}

// CollectHelpCommandPaths walks the Cobra command tree and returns every command
// path that should receive a help recording. Hidden commands and the record
// command subtree are skipped.
func CollectHelpCommandPaths(root *cobra.Command) []HelpCommand {
	var out []HelpCommand
	collectHelpCommandPaths(root, nil, &out)
	return out
}

func collectHelpCommandPaths(cmd *cobra.Command, prefix []string, out *[]HelpCommand) {
	for _, child := range cmd.Commands() {
		if child.Hidden {
			continue
		}
		name := commandNameFromUse(child.Use)
		if name == "" {
			continue
		}
		path := append(append([]string{}, prefix...), name)
		if shouldSkipHelpPath(path) {
			continue
		}
		pathStr := strings.Join(path, " ")
		*out = append(*out, HelpCommand{
			Path: pathStr,
			Slug: pathToSlug(pathStr),
		})
		collectHelpCommandPaths(child, path, out)
	}
}

func shouldSkipHelpPath(path []string) bool {
	return len(path) > 0 && path[0] == "record"
}

func commandNameFromUse(use string) string {
	parts := strings.Fields(use)
	if len(parts) == 0 {
		return ""
	}
	return parts[0]
}

func pathToSlug(path string) string {
	return strings.ReplaceAll(path, " ", "-")
}
