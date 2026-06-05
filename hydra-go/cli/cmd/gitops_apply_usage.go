package cmd

import (
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

func init() {
	cobra.AddTemplateFunc("clusterApplyGroupedFlagUsages", clusterApplyGroupedFlagUsagesTemplate)
}

// clusterApplyGroupedFlagUsagesTemplate is used only by the cluster apply usage template.
func clusterApplyGroupedFlagUsagesTemplate(data interface{}) string {
	cmd, ok := data.(*cobra.Command)
	if !ok || cmd == nil {
		return ""
	}
	return clusterApplyGroupedFlagUsages(cmd)
}

// configureClusterApplyUsage sets a grouped flag layout for `hydra gitops apply` help/usage:
// General flags, then optional apply toggles paired with --no-* opt-outs, then the bootstrap bundle.
func configureClusterApplyUsage(cmd *cobra.Command) {
	cmd.Flags().SortFlags = false
	cmd.SetUsageTemplate(clusterApplyUsageTemplate)
}

// clusterApplyUsageTemplate matches cobra's default usage template except the local Flags block.
const clusterApplyUsageTemplate = `{{HeadingStyle "Usage:"}}{{if .Runnable}}
  {{UseLineStyle .UseLine}}{{end}}{{if .HasAvailableSubCommands}}
  {{ExecStyle .CommandPath}} [command]{{end}}{{if gt (len .Aliases) 0}}

{{HeadingStyle "Aliases:"}}
  {{AliasStyle .NameAndAliases}}{{end}}{{if .HasExample}}

{{HeadingStyle "Examples:"}}
{{ExampleStyle .Example}}{{end}}{{if .HasAvailableSubCommands}}{{$cmds := .Commands}}{{if eq (len .Groups) 0}}

{{HeadingStyle "Available Commands:"}}{{range $cmds}}{{if (or .IsAvailableCommand (eq .Name "help"))}}
  {{rpad (CommandStyle .Name) (sum .NamePadding 12)}} {{CmdShortStyle .Short}}{{end}}{{end}}{{else}}{{range $group := .Groups}}

{{HeadingStyle .Title}}{{range $cmds}}{{if (and (eq .GroupID $group.ID) (or .IsAvailableCommand (eq .Name "help")))}}
  {{rpad (CommandStyle .Name) (sum .NamePadding 12)}} {{CmdShortStyle .Short}}{{end}}{{end}}{{end}}{{if not .AllChildCommandsHaveGroup}}

{{HeadingStyle "Additional Commands:"}}{{range $cmds}}{{if (and (eq .GroupID "") (or .IsAvailableCommand (eq .Name "help")))}}
  {{rpad (CommandStyle .Name) (sum .NamePadding 12)}} {{CmdShortStyle .Short}}{{end}}{{end}}{{end}}{{end}}{{end}}{{if .HasAvailableLocalFlags}}

{{HeadingStyle "Flags:"}}
{{FlagStyle (clusterApplyGroupedFlagUsages .)}}{{end}}{{if .HasAvailableInheritedFlags}}

{{HeadingStyle "Global Flags:"}}
{{FlagStyle (trimTrailingWhitespaces .InheritedFlags.FlagUsages)}}{{end}}{{if .HasHelpSubCommands}}

{{HeadingStyle "Additional help topics:"}}{{range .Commands}}{{if .IsAdditionalHelpTopicCommand}}
  {{rpad (CommandStyle .CommandPath) (sum .CommandPathPadding 12)}} {{CmdShortStyle .Short}}{{end}}{{end}}{{end}}{{if .HasAvailableSubCommands}}

Use "{{ExecStyle .CommandPath}} [command] --help" for more information about a command.{{end}}
`

// clusterApplyOptionalBehaviorFlagOrder lists enable/opt-out pairs (and related backup flags) for non-bootstrap applies.
var clusterApplyOptionalBehaviorFlagOrder = []string{
	"sops-decode", "no-sops-decode",
	"down-scaled", "no-down-scaled",
	"scale-up", "no-scale-up",
	"orphan-scale-down", "no-orphan-scale-down",
	"bootstrap-guard", "no-bootstrap-guard",
	"bootstrap-clones", "no-bootstrap-clones",
	"backup-restore", "skip-backup-restore", "no-backup-restore",
	"disable-webhooks", "no-disable-webhooks",
}

// clusterApplyBootstrapFlagOrder lists bootstrap-only and bootstrap-related flags together.
var clusterApplyBootstrapFlagOrder = []string{
	"bootstrap",
	"skip-bootstrap-guard",
	"skip-ref-checks",
	"sync",
}

func clusterApplyGroupedFlagUsages(cmd *cobra.Command) string {
	claimed := make(map[string]struct{}, len(clusterApplyOptionalBehaviorFlagOrder)+len(clusterApplyBootstrapFlagOrder))
	for _, name := range clusterApplyOptionalBehaviorFlagOrder {
		claimed[name] = struct{}{}
	}
	for _, name := range clusterApplyBootstrapFlagOrder {
		claimed[name] = struct{}{}
	}

	var general []string
	cmd.LocalFlags().VisitAll(func(f *pflag.Flag) {
		if _, skip := claimed[f.Name]; !skip {
			general = append(general, f.Name)
		}
	})

	var b strings.Builder
	writeFlagSection(&b, "General", cmd, general)
	writeFlagSection(&b, "Optional apply behaviors (enable / bootstrap opt-out pairs)", cmd, clusterApplyOptionalBehaviorFlagOrder)
	writeFlagSection(&b, "Bootstrap bundle", cmd, clusterApplyBootstrapFlagOrder)
	return strings.TrimRight(b.String(), "\n")
}

func writeFlagSection(b *strings.Builder, title string, cmd *cobra.Command, flagNames []string) {
	fs := pflag.NewFlagSet(title, pflag.ContinueOnError)
	fs.SortFlags = false
	for _, name := range flagNames {
		if f := cmd.Flags().Lookup(name); f != nil {
			fs.AddFlag(f)
		}
	}
	if !fs.HasFlags() {
		return
	}
	if b.Len() > 0 {
		b.WriteString("\n")
	}
	b.WriteString(title)
	b.WriteString(":\n")
	b.WriteString(strings.TrimRight(fs.FlagUsages(), "\n"))
	b.WriteString("\n")
}
