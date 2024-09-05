package shar

import (
	"fmt"
	"github.com/spf13/cobra"
)

// Cmd is the cobra command object
var Cmd = &cobra.Command{
	Use:   "shar",
	Short: "Start a SHAR server instance",
	Long:  ``,
	RunE:  run,
	//Args:  cobra.MatchAll(cobra.ExactArgs(1), cobra.OnlyValidArgs),
}

func run(cmd *cobra.Command, args []string) error {
	if err := cmd.ValidateArgs(args); err != nil {
		return fmt.Errorf("invalid arguments: %w", err)
	}
	return nil
}
