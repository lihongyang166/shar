package main

import (
	"gitlab.com/shar-workflow/shar/common/logx"
	"gitlab.com/shar-workflow/shar/server/config"
	"gitlab.com/shar-workflow/shar/server/server"
	"log"
	"log/slog"
)

func main() {
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

	shutdonwFn := logx.SetDefault(cfg.LogHandler, lev, addSource, "shar")
	defer func() {
		er := shutdonwFn()
		if er != nil {
			slog.Warn("error during logging shutdown", slog.Any("error", er))
		}
	}()

	if err != nil {
		panic(err)
	}
	svr := server.New(server.Concurrency(cfg.Concurrency), server.NatsUrl(cfg.NatsURL), server.GrpcPort(cfg.Port))
	svr.Listen()
}
