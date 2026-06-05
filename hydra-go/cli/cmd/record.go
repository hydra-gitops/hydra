package cmd

import (
	"os"

	"hydra-gitops.org/hydra/hydra-go/base/record"
	"github.com/spf13/cobra"
)

func newRecordCommand(root *cobra.Command) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "record",
		Short: "Record terminal casts for documentation",
		Long: `Record asciicast v3 files for Hydra CLI documentation.

Subcommands discover Hydra commands automatically and capture their
help output in a pseudo-terminal.`,
	}

	cmd.AddCommand(newRecordHelpCommand(root))
	cmd.AddCommand(newRecordAllCommand(root))
	return cmd
}

func newRecordAllCommand(root *cobra.Command) *cobra.Command {
	var opts recordHelpCLIParams

	cmd := &cobra.Command{
		Use:   "all",
		Short: "Record all documentation casts",
		Long: `Record all documentation casts. Currently this runs the same help
recordings as "hydra record help"; additional cast types will be added later.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runRecordHelp(root, opts)
		},
	}

	addRecordCommonFlags(cmd, &opts)
	return cmd
}

func newRecordHelpCommand(root *cobra.Command) *cobra.Command {
	var opts recordHelpCLIParams

	cmd := &cobra.Command{
		Use:   "help",
		Short: "Record help output for all Hydra CLI commands",
		Long: `Discover all Hydra CLI commands and record "hydra <command> --help"
for each one as an asciicast under hydra/docs/asciinema/help/.

Each recording starts with a generic "$ " shell prompt. Captured terminal output is
written to stdout by default; use --no-mirror-output to disable.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runRecordHelp(root, opts)
		},
	}

	addRecordCommonFlags(cmd, &opts)
	return cmd
}

type recordHelpCLIParams struct {
	outputDir      string
	hydraBin       string
	noMirrorOutput bool
}

func addRecordCommonFlags(cmd *cobra.Command, opts *recordHelpCLIParams) {
	cmd.Flags().StringVar(&opts.outputDir, "output-dir", defaultRecordHelpOutputDir(),
		"Directory for .cast files (one file per command path)")
	cmd.Flags().StringVar(&opts.hydraBin, "hydra-bin", defaultHydraBin(),
		"Hydra executable used inside recordings")
	cmd.Flags().BoolVar(&opts.noMirrorOutput, "no-mirror-output", false,
		"Do not write captured recording output to stdout")
}

func defaultRecordHelpOutputDir() string {
	if _, err := os.Stat("hydra/docs"); err == nil {
		return "hydra/docs/asciinema/help"
	}
	if _, err := os.Stat("docs"); err == nil {
		return "docs/asciinema/help"
	}
	return "hydra/docs/asciinema/help"
}

func defaultHydraBin() string {
	if len(os.Args) > 0 && os.Args[0] != "" {
		return os.Args[0]
	}
	return "hydra"
}

func runRecordHelp(root *cobra.Command, opts recordHelpCLIParams) error {
	return record.RecordAllHelp(record.HelpRecordOptions{
		Root:         root,
		HydraBin:     opts.hydraBin,
		OutputDir:    opts.outputDir,
		MirrorOutput: !opts.noMirrorOutput,
	})
}
