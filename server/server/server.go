package server

import (
	"context"
	"fmt"
	"gitlab.com/shar-workflow/shar/internal/server/workflow"
	"log/slog"
	"net"
	"os"
	"os/signal"
	"syscall"

	"github.com/nats-io/nats.go/jetstream"
	"gitlab.com/shar-workflow/shar/common"
	"gitlab.com/shar-workflow/shar/common/telemetry"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/stdout/stdouttrace"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"
	"go.opentelemetry.io/otel/trace/noop"

	"github.com/hashicorp/go-version"
	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/nats-io/nats.go"
	"gitlab.com/shar-workflow/shar/common/authn"
	"gitlab.com/shar-workflow/shar/common/authz"
	version2 "gitlab.com/shar-workflow/shar/common/version"
	"gitlab.com/shar-workflow/shar/model"
	"gitlab.com/shar-workflow/shar/server/api"
	"gitlab.com/shar-workflow/shar/server/health"
	gogrpc "google.golang.org/grpc"
	grpcHealth "google.golang.org/grpc/health/grpc_health_v1"
)

// Server is the shar server type responsible for hosting the SHAR API.
type Server struct {
	sig                     chan os.Signal
	healthServiceEnabled    bool
	healthService           *health.Checker
	grpcServer              *gogrpc.Server
	api                     *api.Endpoints
	ephemeralStorage        bool
	panicRecovery           bool
	allowOrphanServiceTasks bool
	concurrency             int
	apiAuthorizer           authz.APIFunc
	apiAuthenticator        authn.Check
	SharVersion             *version.Version
	natsUrl                 string
	grpcPort                int
	conn                    *nats.Conn
	telemetryConfig         telemetry.Config
	tr                      trace.Tracer
	noSplash                bool
}

// New creates a new SHAR server.
// Leave the exporter nil if telemetry is not required
func New(options ...Option) *Server {
	currentVer, err := version.NewVersion(version2.Version)
	if err != nil {
		panic(err)
	}
	s := &Server{
		SharVersion:             currentVer,
		sig:                     make(chan os.Signal, 10),
		healthService:           health.New(),
		panicRecovery:           true,
		allowOrphanServiceTasks: true,
		healthServiceEnabled:    true,
		concurrency:             6,
		noSplash:                false,
	}

	for _, i := range options {
		i.configure(s)
	}

	if s.apiAuthorizer == nil {
		slog.Warn("No AuthZ set")
		s.apiAuthorizer = noopAuthZ
	}
	if s.apiAuthenticator == nil {
		slog.Warn("No AuthN set")
		s.apiAuthenticator = noopAuthN
	}

	if !s.noSplash {
		// Show some details about the newly configured server:
		fmt.Printf(`
	███████╗██╗  ██╗ █████╗ ██████╗
	██╔════╝██║  ██║██╔══██╗██╔══██╗
	███████╗███████║███████║██████╔╝
	╚════██║██╔══██║██╔══██║██╔══██╗
	███████║██║  ██║██║  ██║██║  ██║
	╚══════╝╚═╝  ╚═╝╚═╝  ╚═╝╚═╝  ╚═╝
	` + "\n")

		s.Details()
	}
	return s
}

func noopAuthN(_ context.Context, _ *model.ApiAuthenticationRequest) (*model.ApiAuthenticationResponse, error) {
	return &model.ApiAuthenticationResponse{
		User:          "anonymous",
		Authenticated: true,
	}, nil
}

func noopAuthZ(_ context.Context, _ *model.ApiAuthorizationRequest) (*model.ApiAuthorizationResponse, error) {
	return &model.ApiAuthorizationResponse{
		Authorized: true,
	}, nil
}

// The following variables are set by -ldflags at build time.
var (
	VersionTag string
	CommitHash string
	BuildDate  string
)

// Details prints the details to stdout of the current SHAR server.
func (s *Server) Details() {
	t := table.NewWriter()
	t.SetStyle(table.StyleLight)
	t.SetOutputMirror(os.Stdout)
	t.AppendHeader(table.Row{"SHAR SERVER CONFIGURATION", "VALUE"})
	t.Style().Options.SeparateRows = true
	t.AppendRows([]table.Row{
		{"Version                ", version2.Version},
		{"Build Time             ", BuildDate},
		{"Commit SHA             ", CommitHash},
		{"Nats URL               ", s.natsUrl},
		{"Nats Client Version    ", version2.NatsVersion},
		{"Concurrency            ", s.concurrency},
		{"Ephemeral Storage      ", s.ephemeralStorage},
		{"Panic Recovery         ", s.panicRecovery},
		{"AllowOrphanServiceTasks", s.allowOrphanServiceTasks},
		{"Grpc Port              ", s.grpcPort},
		{"Telemetry Enabled      ", s.telemetryConfig.Enabled},
		{"Telemetry Endpoint     ", s.telemetryConfig.Endpoint},
	}, table.RowConfig{AutoMerge: false})
	t.AppendSeparator()
	t.Render()
}

// Listen starts the GRPC server for both serving requests, and the GRPC health endpoint.
func (s *Server) Listen() error {
	// Set up telemetry for the server
	setupTelemetry(s)

	// Capture errors and cancel signals
	errs := make(chan error)

	// Capture SIGTERM and SIGINT
	signal.Notify(s.sig, syscall.SIGTERM, syscall.SIGINT)

	if s.healthServiceEnabled {
		// Create health server and expose on GRPC
		lis, err := net.Listen("tcp", fmt.Sprintf(":%d", s.grpcPort))
		if err != nil {
			slog.Error("listen", "error", err, slog.Int64("grpcPort", int64(s.grpcPort)))
			panic(err)
		}

		s.grpcServer = gogrpc.NewServer()
		if err := registerServer(s.grpcServer, s.healthService); err != nil {
			slog.Error("register grpc health server", "error", err, slog.Int64("grpcPort", int64(s.grpcPort)))
			panic(err)
		}

		// Start health server
		go func() {
			if err := s.grpcServer.Serve(lis); err != nil {
				errs <- err
			}
			close(errs)
		}()
		slog.Info("shar grpc health started")
	} else {
		// Create private health server
		s.healthService.SetStatus(grpcHealth.HealthCheckResponse_NOT_SERVING)
	}

	nc, err := s.ConnectNats(s.natsUrl, s.ephemeralStorage)
	if err != nil {
		return fmt.Errorf("connect nats: %w", err)
	}

	wfe, err := s.createWorkflowEngine(nc, s.allowOrphanServiceTasks)
	if err != nil {
		return fmt.Errorf("create workflow entry: %w", err)
	}

	a, err := api.New(nc, wfe, s.panicRecovery, s.apiAuthorizer, s.apiAuthenticator)
	if err != nil {
		return fmt.Errorf("create api: %w", err)
	}
	s.api = a
	s.healthService.SetStatus(grpcHealth.HealthCheckResponse_SERVING)

	if err := s.api.Listen(); err != nil {
		panic(err)
	}
	// Log or exit
	select {
	case err := <-errs:
		if err != nil {
			slog.Error("fatal error", "error", err)
			panic("fatal error")
		}
	case <-s.sig:
		s.Shutdown()
	}
	return nil
}

func setupTelemetry(s *Server) {
	traceName := "shar"
	switch s.telemetryConfig.Endpoint {
	case "console":
		exporter, err := stdouttrace.New(stdouttrace.WithPrettyPrint())
		if err != nil {
			err := fmt.Errorf("create stdouttrace exporter: %w", err)
			slog.Error(err.Error())
			otel.SetTracerProvider(noop.NewTracerProvider())
			goto setProvider
		}
		batchSpanProcessor := sdktrace.NewBatchSpanProcessor(exporter)
		tp := sdktrace.NewTracerProvider(
			sdktrace.WithSampler(sdktrace.AlwaysSample()),
			sdktrace.WithSpanProcessor(batchSpanProcessor),
		)
		otel.SetTracerProvider(tp)
	default:
		otel.SetTracerProvider(noop.NewTracerProvider())
	}
setProvider:
	s.tr = otel.GetTracerProvider().Tracer(traceName, trace.WithInstrumentationVersion(version2.Version))
}

// Shutdown gracefully shuts down the GRPC server, and requests that
func (s *Server) Shutdown() {
	s.healthService.SetStatus(grpcHealth.HealthCheckResponse_NOT_SERVING)

	s.api.Shutdown()
	if s.healthServiceEnabled {
		s.grpcServer.GracefulStop()
		slog.Info("shar grpc health stopped")
	}
}

// GetEndPoint will return the URL of the GRPC health endpoint for the shar server
func (s *Server) GetEndPoint() string {
	return "TODO" // can we discover the grpc endpoint listen address??
}

// ConnectNats establishes a connection to the NATS server using the given URL.
// It also creates a separate transactional NATS connection.
// It checks the NATS server version and obtains the JetStream account information.
// It returns the NATS connection configuration that includes the NATS connection,
// transactional NATS connection, and the storage type for JetStream.
//
// Parameters:
// - natsURL: The URL of the NATS server.
// - ephemeral: A flag indicating whether to use ephemeral storage for JetStream.
//
// Returns:
// - NatsConnConfiguration: The NATS connection configuration.
// - error: An error if the connection or account retrieval fails.
func (s *Server) ConnectNats(natsURL string, ephemeral bool) (*workflow.NatsConnConfiguration, error) {
	// TODO why do we need a separate txConn?
	conn, err := nats.Connect(natsURL)
	if err != nil {
		slog.Error("connect to NATS", slog.String("error", err.Error()), slog.String("url", natsURL))
		return nil, fmt.Errorf("connect to NATS: %w", err)
	}
	txConn, err := nats.Connect(natsURL)
	if err != nil {
		slog.Error("connect to NATS", slog.String("error", err.Error()), slog.String("url", natsURL))
		return nil, fmt.Errorf("connect to NATS: %w", err)
	}
	ctx := context.Background()
	if err := common.CheckVersion(ctx, txConn); err != nil {
		return nil, fmt.Errorf("check NATS version: %w", err)
	}
	if js, err := conn.JetStream(); err != nil {
		return nil, fmt.Errorf("connect to JetStream: %w", err)
	} else {
		if _, err := js.AccountInfo(); err != nil {
			return nil, fmt.Errorf("get NATS account information: %w", err)
		}
	}
	store := jetstream.FileStorage
	if ephemeral {
		store = jetstream.MemoryStorage
	}
	return &workflow.NatsConnConfiguration{
		Conn:        conn,
		TxConn:      txConn,
		StorageType: store,
	}, nil
}

func (s *Server) createWorkflowEngine(nc *workflow.NatsConnConfiguration, allowOrphanServiceTasks bool) (*workflow.Engine, error) {
	ns, err := workflow.New(nc, s.concurrency, allowOrphanServiceTasks, s.telemetryConfig)
	if err != nil {
		slog.Error("create NATS KV store", slog.String("error", err.Error()))
		return nil, fmt.Errorf("create NATS KV store: %w", err)
	}
	return ns, nil
}

// Ready returns true if the SHAR server is servicing API calls.
func (s *Server) Ready() bool {
	if s.healthService != nil {
		return s.healthService.GetStatus() == grpcHealth.HealthCheckResponse_SERVING
	} else {
		return false
	}
}

func (s *Server) setSharVersion(version *version.Version) {
	s.SharVersion = version
}

func registerServer(s *gogrpc.Server, hs *health.Checker) error {
	hs.SetStatus(grpcHealth.HealthCheckResponse_NOT_SERVING)
	grpcHealth.RegisterHealthServer(s, hs)
	return nil
}
