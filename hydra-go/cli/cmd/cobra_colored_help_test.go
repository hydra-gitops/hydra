package cmd

import (
	"testing"

	"github.com/fatih/color"
	"github.com/stretchr/testify/assert"
)

func TestStyleFlagTokens_enumPipeListUnchanged(t *testing.T) {
	cf := color.New(color.Bold)
	cf.DisableColor()

	line := "default|manual|auto|prevent|keep-or-manual|keep-or-auto|keep-or-prevent|keep-or-default|down-scaled"
	assert.Equal(t, line, styleFlagTokens(line, cf), "hyphens inside pipe-separated values must not start flag styling")
}

func TestStyleFlagTokens_flagColumnAndEnumOnSameLine(t *testing.T) {
	cf := color.New(color.Bold)
	cf.DisableColor()

	line := "      --sync-window string    ArgoCD AppProject sync policy: default|manual|auto|prevent|keep-or-manual|down-scaled"
	got := styleFlagTokens(line, cf)
	assert.Contains(t, got, "--sync-window")
	assert.Contains(t, got, "keep-or-manual")
	assert.Contains(t, got, "down-scaled")
}

func TestStyleFlagTokens_shortAndLongInFlagColumn(t *testing.T) {
	cf := color.New(color.Bold)
	cf.DisableColor()

	line := "  -f, --file string    path"
	got := styleFlagTokens(line, cf)
	assert.Contains(t, got, "-f")
	assert.Contains(t, got, "--file")
}

func TestStyleFlagTokens_bootstrapGuardLineMentionsThreeFlags(t *testing.T) {
	cf := color.New(color.Bold, color.FgHiWhite)
	cf.DisableColor()

	line := "      --bootstrap-guard       Enforce bootstrap-guard ref rules: fail when guarded resources are present unless using --bootstrap or --skip-bootstrap-guard"
	got := styleFlagTokens(line, cf)
	assert.Contains(t, got, "--bootstrap-guard")
	assert.Contains(t, got, "--bootstrap")
	assert.Contains(t, got, "--skip-bootstrap-guard")
}

func TestLongHelpStyle_sectionHeadingsAndFlags(t *testing.T) {
	section := color.New(color.FgHiCyan, color.Bold)
	cf := color.New(color.Bold, color.FgHiWhite)
	section.DisableColor()
	cf.DisableColor()

	raw := `Render Helm templates and apply.

App IDs support glob-style wildcard matching:
  * matches

Examples:
  prod.* all

Use --exclude-app to narrow.`

	got := longHelpStyleText(raw, section, cf)
	assert.Contains(t, got, "App IDs support glob-style wildcard matching:")
	assert.Contains(t, got, "Examples:")
	assert.Contains(t, got, "--exclude-app")
	assert.Contains(t, got, "Render Helm templates")
}
