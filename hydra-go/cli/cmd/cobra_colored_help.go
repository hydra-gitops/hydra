package cmd

import (
	"regexp"
	"strings"

	"github.com/fatih/color"
	cc "github.com/ivanpirog/coloredcobra"
	"github.com/spf13/cobra"
)

func init() {
	// Defaults so help templates work before ApplyColoredCobraHelp (e.g. tests that call
	// Execute on a subcommand without going through ExecuteArgs). Init overwrites these.
	cobra.AddTemplateFunc("HeadingStyle", func(s string) string { return s })
	cobra.AddTemplateFunc("CommandStyle", func(s string) string { return s })
	cobra.AddTemplateFunc("sum", func(a, b int) int { return a + b })
	cobra.AddTemplateFunc("CmdShortStyle", func(s string) string { return s })
	cobra.AddTemplateFunc("ExecStyle", func(s string) string { return s })
	cobra.AddTemplateFunc("UseLineStyle", func(s string) string { return s })
	cobra.AddTemplateFunc("FlagStyle", func(s string) string { return s })
	cobra.AddTemplateFunc("AliasStyle", func(s string) string { return s })
	cobra.AddTemplateFunc("ExampleStyle", func(s string) string { return s })
	cobra.AddTemplateFunc("LongHelpStyle", func(s string) string { return s })
}

// hydraDefaultHelpTemplate matches cobra's defaultHelpTemplate but runs Long/Short through
// LongHelpStyle so intro text and embedded section titles (e.g. "Examples:") get the same
// highlighting as Usage headings; flag mentions in prose are styled like flag usages.
const hydraDefaultHelpTemplate = `{{with (or .Long .Short)}}{{LongHelpStyle (trimTrailingWhitespaces .)}}

{{end}}{{if or .Runnable .HasSubCommands}}{{.UsageString}}{{end}}`

// ApplyColoredCobraHelp enables colored usage/help for the command tree. It uses fatih/color's
// default behavior (TTY / NO_COLOR / TERM=dumb) via coloredcobra.
func ApplyColoredCobraHelp(rootCmd *cobra.Command) {
	cc.Init(&cc.Config{
		RootCmd:       rootCmd,
		Headings:      cc.HiCyan + cc.Bold,
		Commands:      cc.HiYellow + cc.Bold,
		CmdShortDescr: cc.Cyan,
		ExecName:      cc.Bold,
		Flags:         cc.Bold,
		FlagsDescr:    cc.White,
		Aliases:       cc.Bold,
		Example:       cc.Italic,
	})
	// coloredcobra matches (--?\S+) and wrongly highlights "-or-manual" inside "keep-or-manual".
	// Only treat "--long-opt" / "-short" as flags when the leading '-' starts the token after
	// line start, whitespace, or '|' (enum separators).
	cobra.AddTemplateFunc("FlagStyle", newFlagStyleFunc())
	cobra.AddTemplateFunc("LongHelpStyle", newLongHelpStyleFunc())
	rootCmd.SetHelpTemplate(hydraDefaultHelpTemplate)
}

// longSectionHeadingLine matches a whole-line section title ending with ':' (no '.' before ':'),
// e.g. "Examples:" or "App IDs support glob-style wildcard matching:" — not normal sentences.
var longSectionHeadingLine = regexp.MustCompile(`^(\s*)([A-Za-z][^\n.:]*:\s*)$`)

// flagTokenRE matches a CLI flag token only when '-' begins the option after ^, whitespace, or '|'.
var flagTokenRE = regexp.MustCompile(`(^|[\s|])(--[a-zA-Z0-9][\w-]*|-[a-zA-Z0-9][a-zA-Z0-9_-]*)`)

func newFlagStyleFunc() func(string) string {
	// Bold + bright white so flags read clearly on dark terminals (not plain bold only).
	cf := color.New(color.Bold, color.FgHiWhite)
	cfd := color.New(color.FgWhite)
	return func(s string) string {
		return applyFlagStyle(s, cf, cfd)
	}
}

func newLongHelpStyleFunc() func(string) string {
	section := color.New(color.FgHiCyan, color.Bold)
	cf := color.New(color.Bold, color.FgHiWhite)
	return func(s string) string {
		return longHelpStyleText(s, section, cf)
	}
}

func longHelpStyleText(s string, section, cf *color.Color) string {
	lines := strings.Split(s, "\n")
	for i, line := range lines {
		if m := longSectionHeadingLine.FindStringSubmatch(line); m != nil {
			title := strings.TrimSpace(m[2])
			lines[i] = m[1] + section.Sprint(title)
			continue
		}
		lines[i] = styleFlagTokens(line, cf)
	}
	return strings.Join(lines, "\n")
}

func applyFlagStyle(s string, cf, cfd *color.Color) string {
	lines := strings.Split(s, "\n")
	for k := range lines {
		if cf != nil {
			lines[k] = styleFlagTokens(lines[k], cf)
		}
		if cfd == nil {
			continue
		}
		re := regexp.MustCompile(`\s{2,}`)
		spl := re.Split(lines[k], -1)
		if len(spl) != 3 {
			continue
		}
		lines[k] = strings.Replace(lines[k], spl[2], cfd.Sprint(spl[2]), 1)
	}
	return strings.Join(lines, "\n")
}

// maxFlagTokenHighlights caps how many flag-like tokens we style per line. The original
// coloredcobra limit was 2 (to avoid painting enum lines); our stricter flagTokenRE already
// skips "keep-or-manual" etc., but a single help line can still mention several real flags
// (e.g. "--bootstrap-guard ... unless using --bootstrap or --skip-bootstrap-guard").
const maxFlagTokenHighlights = 32

// styleFlagTokens applies bold + color to flag tokens per line, using boundaries so values
// like "keep-or-manual" or "down-scaled" are not split at inner hyphens.
func styleFlagTokens(line string, cf *color.Color) string {
	for n := 0; n < maxFlagTokenHighlights; n++ {
		m := flagTokenRE.FindStringSubmatchIndex(line)
		if m == nil {
			break
		}
		sub := flagTokenRE.FindStringSubmatch(line)
		if len(sub) != 3 {
			break
		}
		prefix, flagText := sub[1], sub[2]
		styled := prefix + cf.Sprint(flagText)
		line = line[:m[0]] + styled + line[m[1]:]
	}
	return line
}
