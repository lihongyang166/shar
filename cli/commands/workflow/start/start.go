package start

import (
	"context"
	"fmt"
	"github.com/nats-io/nats.go/jetstream"
	"gitlab.com/shar-workflow/shar/cli/util"
	"gitlab.com/shar-workflow/shar/client"
	"gitlab.com/shar-workflow/shar/common/logx"
	"gitlab.com/shar-workflow/shar/common/namespace"
	"gitlab.com/shar-workflow/shar/common/subj"
	"log/slog"

	"github.com/nats-io/nats.go"
	"github.com/spf13/cobra"
	"gitlab.com/shar-workflow/shar/cli/flag"
	"gitlab.com/shar-workflow/shar/cli/output"
	"gitlab.com/shar-workflow/shar/common"
	"gitlab.com/shar-workflow/shar/common/valueparsing"
	"gitlab.com/shar-workflow/shar/model"
	"gitlab.com/shar-workflow/shar/server/messages"
	"google.golang.org/protobuf/proto"
)

// Cmd is the cobra command object
var Cmd = &cobra.Command{
	Use:   "start",
	Short: "Starts a new execution",
	Long:  `shar workflow start "ProcessID"`,
	RunE:  run,
	Args:  cobra.MatchAll(cobra.ExactArgs(1), cobra.OnlyValidArgs),
}

func run(cmd *cobra.Command, args []string) error {
	if err := cmd.ValidateArgs(args); err != nil {
		return fmt.Errorf("invalid arguments: %w", err)
	}
	vars := model.NewVars()
	var err error
	if len(flag.Value.Vars) > 0 {
		vars, err = valueparsing.Parse(flag.Value.Vars)
		if err != nil {
			return fmt.Errorf("parse flags: %w", err)
		}
	}

	ctx := context.Background()
	shar := util.GetClient()
	if err := shar.Dial(ctx, flag.Value.Server); err != nil {
		return fmt.Errorf("dialling server: %w", err)
	}
	executionID, wfID, err := shar.LaunchProcess(ctx, client.LaunchParams{ProcessID: args[0], Vars: vars})
	if err != nil {
		return fmt.Errorf("workflow launch failed: %w", err)
	}

	if flag.Value.DebugTrace {
		// Connect to a server
		nc, _ := nats.Connect(nats.DefaultURL)

		// get Jetstream
		js, err := jetstream.New(nc)
		if err != nil {
			panic(err)
		}

		if _, err := js.CreateOrUpdateConsumer(ctx, "WORKFLOW", jetstream.ConsumerConfig{
			Durable:       "Tracing",
			Description:   "Sequential Trace Consumer",
			DeliverPolicy: jetstream.DeliverAllPolicy,
			FilterSubject: subj.NS(messages.WorkflowStateAll, namespace.Default),
			AckPolicy:     jetstream.AckExplicitPolicy,
		}); err != nil {
			panic(err)
		}

		ctx = context.Background()
		closer := make(chan struct{})
		workflowMessages := make(chan jetstream.Msg)

		err = common.Process(ctx, js, "WORKFLOW_TELEMETRY", "trace", closer, subj.NS(messages.WorkflowStateAll, "*"), "Tracing", 1, nil, func(ctx context.Context, log *slog.Logger, msg jetstream.Msg) (bool, error) {
			workflowMessages <- msg
			return true, nil
		}, nil)
		if err != nil {
			return fmt.Errorf("starting debug trace processing: %w", err)
		}

		for msg := range workflowMessages {
			var state = model.WorkflowState{}
			err := proto.Unmarshal(msg.Data(), &state)
			if err != nil {
				log := logx.FromContext(ctx)
				log.Error("unmarshal message", "error", err)
				return fmt.Errorf("unmarshal status trace message: %w", err)
			}
			//if state.WorkflowInstanceId == executionID {
			//TODO: Re- implement
			//	output.Current.OutputExecutionStatus(executionID, []*model.WorkflowState{&state})
			//}
			// Check end states once they are implemented
			// if state.State == "" {
			// 	close(closer)
			// 	close(workflowMessages)
			// }
		}
	}
	output.Current.OutputStartWorkflowResult(executionID, wfID)
	return nil
}

func init() {
	Cmd.PersistentFlags().BoolVarP(&flag.Value.DebugTrace, flag.DebugTrace, flag.DebugTraceShort, false, "enable debug trace for selected workflow")
	Cmd.PersistentFlags().StringSliceVarP(&flag.Value.Vars, flag.Vars, flag.VarsShort, []string{}, "pass variables to given workflow, eg --vars \"orderId:int(78),serviceId:string(hello)\"")
}
