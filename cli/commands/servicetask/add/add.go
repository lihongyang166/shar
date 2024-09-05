package add

import (
	"context"
	"fmt"
	"gitlab.com/shar-workflow/shar/cli/output"

	"github.com/spf13/cobra"
	"gitlab.com/shar-workflow/shar/cli/flag"
	"gitlab.com/shar-workflow/shar/cli/util"
	"gitlab.com/shar-workflow/shar/client/taskutil"
)

// Cmd is the cobra command object
var Cmd = &cobra.Command{
	Use:   "add",
	Short: "Adds a service task",
	Long:  ``,
	RunE:  run,
	Args:  cobra.ExactArgs(1),
}

func run(cmd *cobra.Command, args []string) error {
	if err := cmd.ValidateArgs(args); err != nil {
		return fmt.Errorf("invalid arguments: %w", err)
	}

	ctx := context.Background()
	shar := util.GetClient()
	if err := shar.Dial(ctx, flag.Value.Server); err != nil {
		return fmt.Errorf("dialling server: %w", err)
	}
	uid, err := taskutil.LoadTaskFromYamlFile(ctx, shar, args[0])
	if err != nil {
		return fmt.Errorf("load service task: %w", err)
	}
	output.Current.OutputAddTaskResult(uid)
	return nil
}

func init() {
	Cmd.Flags().StringVarP(&flag.Value.CorrelationKey, flag.CorrelationKey, flag.CorrelationKeyShort, "", "a correlation key for the message")
}
