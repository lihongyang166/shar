package commands

import (
	"fmt"
	"github.com/nats-io/nats.go"
	"github.com/spf13/cobra"
	"gitlab.com/shar-workflow/shar/common"
	"gitlab.com/shar-workflow/shar/common/logx"
	show_nats_config "gitlab.com/shar-workflow/shar/server/commands/show-nats-config"
	"gitlab.com/shar-workflow/shar/server/config"
	"gitlab.com/shar-workflow/shar/server/errors"
	"gitlab.com/shar-workflow/shar/server/flags"
	"gitlab.com/shar-workflow/shar/server/server"
	"gitlab.com/shar-workflow/shar/server/server/option"
	"log"
	"log/slog"
	"os"
	"strings"
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
		case "trace":
			lev = errors.TraceLevel
		case "verbose":
			lev = errors.VerboseLevel
		default:
			lev = slog.LevelError
		}

		nc, err := server.ConnectNats(cfg.JetStreamDomain, cfg.NatsURL, []nats.Option{nats.MaxReconnects(cfg.NatsMaxReconnects)}, false)
		if err != nil {
			panic(fmt.Errorf("connect nats: %w", err))
		}

		handlerFactoryFns := map[string]func() slog.Handler{
			"text": func() slog.Handler {
				return common.NewTextHandler(lev, addSource)
			},
			"shar-handler": func() slog.Handler {
				return common.NewSharHandler(common.HandlerOptions{Level: lev}, &common.NatsLogPublisher{Conn: nc.Conn})
			},
		}

		cfgHandlers := strings.Split(cfg.LogHandler, ",")
		handlers := make([]slog.Handler, 0, len(cfgHandlers))
		for _, h := range cfgHandlers {
			handlers = append(handlers, handlerFactoryFns[h]())
		}

		logx.SetDefault("shar", common.NewMultiHandler(handlers))

		options := []option.Option{option.Concurrency(cfg.Concurrency), option.NatsUrl(cfg.NatsURL), option.GrpcPort(cfg.Port)}

		if cfg.JetStreamDomain != "" {
			options = append(options, option.WithJetStreamDomain(cfg.JetStreamDomain))
		}

		var svr *server.Server
		if svr, err = server.New(nc, options...); err != nil {
			panic(fmt.Errorf("creating server: %w", err))
		}

		if err = svr.Listen(); err != nil {
			panic(fmt.Errorf("starting server: %w", err))
		}
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
