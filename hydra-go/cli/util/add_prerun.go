package util

import "github.com/spf13/cobra"

func AddPersistentPreRun(cmd *cobra.Command, persistentPreRun func(cmd *cobra.Command, args []string)) {
	original := cmd.PersistentPreRunE
	cmd.PersistentPreRunE = func(cmd *cobra.Command, args []string) error {
		if original != nil {
			if err := original(cmd, args); err != nil {
				return err
			}
		}
		persistentPreRun(cmd, args)
		return nil
	}
}

func AddPersistentPreRunE(cmd *cobra.Command, persistentPreRunE func(cmd *cobra.Command, args []string) error) {
	original := cmd.PersistentPreRunE
	cmd.PersistentPreRunE = func(cmd *cobra.Command, args []string) error {
		if original != nil {
			if err := original(cmd, args); err != nil {
				return err
			}
		}
		return persistentPreRunE(cmd, args)
	}
}

func AddPreRun(cmd *cobra.Command, preRun func(cmd *cobra.Command, args []string)) {
	original := cmd.PreRunE
	cmd.PreRunE = func(cmd *cobra.Command, args []string) error {
		if original != nil {
			if err := original(cmd, args); err != nil {
				return err
			}
		}
		preRun(cmd, args)
		return nil
	}
}

func AddPreRunE(cmd *cobra.Command, preRunE func(cmd *cobra.Command, args []string) error) {
	original := cmd.PreRunE
	cmd.PreRunE = func(cmd *cobra.Command, args []string) error {
		if original != nil {
			if err := original(cmd, args); err != nil {
				return err
			}
		}
		return preRunE(cmd, args)
	}
}
