package commands

import (
	"github.com/nats-io/nats.go"
	"gitlab.com/shar-workflow/shar/common/logx"
	"gitlab.com/shar-workflow/shar/zen-shar/flag"
	"gitlab.com/shar-workflow/shar/zen-shar/server"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"
)

var sig = make(chan os.Signal, 10)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "zen-shar",
	Short: "ZEN SHAR development server",
	Long:  ``,
	// Uncomment the following line if your bare application
	// has an action associated with it:
	// Run: func(cmd *cobra.Command, args []string) { },
	RunE: run,
}

func run(cmd *cobra.Command, args []string) error {
	// Capture SIGTERM and SIGINT
	signal.Notify(sig, syscall.SIGTERM, syscall.SIGINT)

	setupLogging()
	ns, ss, err := server.GetServers(flag.Value.Concurrency, nil, nil)
	if err != nil {
		panic(err)
	}

	<-sig
	defer ss.Shutdown()
	defer ns.Shutdown()
	return nil
}

// Execute adds all child commands to the root command and sets flag appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	err := rootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}

func init() {
	rootCmd.PersistentFlags().StringVarP(&flag.Value.Server, flag.Server, flag.ServerShort, nats.DefaultURL, "sets the address of a NATS server")
	rootCmd.PersistentFlags().StringVarP(&flag.Value.LogLevel, flag.LogLevel, flag.LogLevelShort, "error", "sets the logging level")
	rootCmd.PersistentFlags().IntVarP(&flag.Value.Concurrency, flag.Concurrency, flag.ConcurrencyShort, 10, "sets the address of a NATS server")
}

func setupLogging() {
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
	logx.SetDefault(lev, addSource, "zen-shar")
}
