package commands

import (
	"github.com/spf13/cobra"
	"gitlab.com/shar-workflow/shar/common/logx"
	show_nats_config "gitlab.com/shar-workflow/shar/server/commands/show-nats-config"
	"gitlab.com/shar-workflow/shar/server/config"
	"gitlab.com/shar-workflow/shar/server/flags"
	"gitlab.com/shar-workflow/shar/server/server"
	"gitlab.com/shar-workflow/shar/server/services/storage"
	"log"
	"log/slog"
	"os"
)

// RootCmd represents the base command when called without any subcommands
var RootCmd = &cobra.Command{
	Use:   "shar",
	Short: "SHAR Server",
	Long:  ``,
	// Uncomment the following line if your bare application
	// has an action associated with it:
	Run: func(cmd *cobra.Command, args []string) {
		cfg, err := config.GetEnvironment()
		if err != nil {
			log.Fatal(err)
		}
		var lev slog.Level
		var addSource bool
		switch cfg.LogLevel {
		case "debug":
			lev = slog.LevelDebug
			addSource = true
		case "info":
			lev = slog.LevelInfo
		case "warn":
			lev = slog.LevelWarn
		default:
			lev = slog.LevelError
		}

		if flags.Value.NatsConfig != "" {
			b, err := os.ReadFile(flags.Value.NatsConfig)
			if err != nil {
				slog.Error("read nats configuration file", slog.String("error", err.Error()))
				os.Exit(1)
			}
			storage.NatsConfig = string(b)
		}

		logx.SetDefault(lev, addSource, "shar")
		if err != nil {
			panic(err)
		}
		svr := server.New(server.Concurrency(cfg.Concurrency), server.NatsUrl(cfg.NatsURL), server.GrpcPort(cfg.Port))
		svr.Listen()
	},
	PersistentPreRun: func(cmd *cobra.Command, args []string) {

	},
}

// Execute adds all child commands to the root command and sets flag appropriately.
// This is called by main.main(). It only needs to happen once to the RootCmd.
func Execute() {
	RootCmd.AddCommand(show_nats_config.RootCmd)
	RootCmd.Flags().StringVar(&flags.Value.NatsConfig, flags.NatsConfig, "", "provides a path to a nats configuration file.  The current config file can be obtained using 'show-nats-config'")
	err := RootCmd.Execute()

	if err != nil {
		os.Exit(1)
	}
}

func init() {
	/*
		RootCmd.AddCommand(bpmn.Cmd)
		RootCmd.AddCommand(execution.Cmd)
		RootCmd.AddCommand(workflow.Cmd)
		RootCmd.AddCommand(message.Cmd)
		RootCmd.AddCommand(usertask.Cmd)
		RootCmd.AddCommand(servicetask.Cmd)
		RootCmd.PersistentFlags().StringVarP(&flag.Value.Server, flag.Server, flag.ServerShort, nats.DefaultURL, "sets the address of a NATS server")
		RootCmd.PersistentFlags().StringVarP(&flag.Value.LogLevel, flag.LogLevel, flag.LogLevelShort, "error", "sets the logging level for the CLI")
		RootCmd.PersistentFlags().BoolVarP(&flag.Value.Json, flag.JsonOutput, flag.JsonOutputShort, false, "sets the CLI output to json")
		var lev slog.Level
		var addSource bool
		switch flag.Value.LogLevel {
		case "debug":
			lev = slog.LevelDebug
			addSource = true
		case "info":
			lev = slog.LevelInfo
		case "warn":
			lev = slog.LevelWarn
		default:
			lev = slog.LevelError
		}
		if flag.Value.Json {
			output.Current = &output.Text{}
		} else {
			output.Current = &output.Json{}
		}
		logx.SetDefault(lev, addSource, "shar-cli")

	*/
}
