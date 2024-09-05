package serve

import (
	"github.com/spf13/cobra"
	"gitlab.com/shar-workflow/shar/cli/commands/serve/all"
	"gitlab.com/shar-workflow/shar/cli/commands/serve/shar"
)

// Cmd is the cobra command object
var Cmd = &cobra.Command{
	Use:   "serve",
	Short: "Starts a local SHAR server instance",
	Long:  ``,
}

func init() {
	Cmd.AddCommand(shar.Cmd)
	Cmd.AddCommand(all.Cmd)
}
