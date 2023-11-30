package main

import (
	"github.com/nats-io/nats.go"
	"gitlab.com/shar-workflow/shar/common"
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

	conn, err := nats.Connect(cfg.NatsURL)
	if err != nil {
		slog.Error("connect to NATS", err, slog.String("url", cfg.NatsURL))
		panic(err)
	}

	var handler slog.Handler
	switch cfg.LogHandler {
	case "shar-handler":
		handler = common.NewSharHandler(common.HandlerOptions{Level: lev}, &common.NatsLogPublisher{Conn: conn})
	default:
		handler = common.NewTextHandler(lev, addSource)
	}

	logx.SetDefault("shar", handler)

	if err != nil {
		panic(err)
	}
	svr := server.New(server.Concurrency(cfg.Concurrency), server.NatsConn(conn), server.NatsUrl(cfg.NatsURL), server.GrpcPort(cfg.Port))
	svr.Listen()
}
