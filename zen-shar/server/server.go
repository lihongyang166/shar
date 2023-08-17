package server

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/nats-io/nats-server/v2/server"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
	"gitlab.com/shar-workflow/shar/common/authn"
	"gitlab.com/shar-workflow/shar/common/authz"
	version2 "gitlab.com/shar-workflow/shar/common/version"
	sharsvr "gitlab.com/shar-workflow/shar/server/server"
	"golang.org/x/exp/slog"
)

type zenOpts struct {
	sharVersion        string
	sharServerImageUrl string
}

// ZenSharOptionApplyFn represents a SHAR Zen Server configuration function
type ZenSharOptionApplyFn func(cfg *zenOpts)

// WithSharVersion artificially sets the reported server version.
func WithSharVersion(ver string) ZenSharOptionApplyFn {
	return func(cfg *zenOpts) {
		cfg.sharVersion = ver
	}
}

func WithSharServerImageUrl(imageUrl string) ZenSharOptionApplyFn {
	return func(cfg *zenOpts) {
		cfg.sharServerImageUrl = imageUrl
	}
}

// GetServers returns a test NATS and SHAR server.
func GetServers(natsHost string, natsPort int, sharConcurrency int, apiAuth authz.APIFunc, authN authn.Check, option ...ZenSharOptionApplyFn) (Server, *server.Server, error) {

	defaults := &zenOpts{sharVersion: version2.Version}
	for _, i := range option {
		i(defaults)
	}
	//wd, err := os.Getwd()
	//if err != nil {
	//	return nil, nil, fmt.Errorf("failed to get working directory: %w", err)
	//}
	nsvr, err := server.NewServer(&server.Options{
		ConfigFile:            "",
		ServerName:            "TestNatsServer",
		Host:                  natsHost,
		Port:                  natsPort,
		ClientAdvertise:       "",
		Trace:                 false,
		Debug:                 false,
		TraceVerbose:          false,
		NoLog:                 false,
		NoSigs:                false,
		NoSublistCache:        false,
		NoHeaderSupport:       false,
		DisableShortFirstPing: false,
		Logtime:               false,
		MaxConn:               0,
		MaxSubs:               0,
		MaxSubTokens:          0,
		Nkeys:                 nil,
		Users:                 nil,
		Accounts: []*server.Account{
			{
				Name:   "sysacc",
				Nkey:   "",
				Issuer: "",
			},
		},
		NoAuthUser:         "",
		SystemAccount:      "sysacc",
		NoSystemAccount:    true,
		Username:           "",
		Password:           "",
		Authorization:      "",
		PingInterval:       0,
		MaxPingsOut:        0,
		HTTPHost:           "",
		HTTPPort:           0,
		HTTPBasePath:       "",
		HTTPSPort:          0,
		AuthTimeout:        0,
		MaxControlLine:     0,
		MaxPayload:         0,
		MaxPending:         0,
		Cluster:            server.ClusterOpts{},
		Gateway:            server.GatewayOpts{},
		LeafNode:           server.LeafNodeOpts{},
		JetStream:          true,
		JetStreamMaxMemory: 0,
		JetStreamMaxStore:  0,
		JetStreamDomain:    "",
		JetStreamExtHint:   "",
		JetStreamKey:       "",
		JetStreamUniqueTag: "",
		JetStreamLimits:    server.JSLimitOpts{},
		//StoreDir:                   path.Join(wd, "_jetstream"),
		JsAccDefaultDomain:         nil,
		Websocket:                  server.WebsocketOpts{},
		MQTT:                       server.MQTTOpts{},
		ProfPort:                   0,
		PidFile:                    "",
		PortsFileDir:               "",
		LogFile:                    "",
		LogSizeLimit:               0,
		Syslog:                     false,
		RemoteSyslog:               "",
		Routes:                     nil,
		RoutesStr:                  "",
		TLSTimeout:                 0,
		TLS:                        false,
		TLSVerify:                  false,
		TLSMap:                     false,
		TLSCert:                    "",
		TLSKey:                     "",
		TLSCaCert:                  "",
		TLSConfig:                  nil,
		TLSPinnedCerts:             nil,
		TLSRateLimit:               0,
		AllowNonTLS:                false,
		WriteDeadline:              0,
		MaxClosedClients:           0,
		LameDuckDuration:           0,
		LameDuckGracePeriod:        0,
		MaxTracedMsgLen:            0,
		TrustedKeys:                nil,
		TrustedOperators:           nil,
		AccountResolver:            nil,
		AccountResolverTLSConfig:   nil,
		AlwaysEnableNonce:          false,
		CustomClientAuthentication: nil,
		CustomRouterAuthentication: nil,
		CheckConfig:                false,
		ConnectErrorReports:        0,
		ReconnectErrorReports:      0,
		Tags:                       nil,
		OCSPConfig:                 nil,
	})
	if err != nil {
		return nil, nil, fmt.Errorf("create a new server instance: %w", err)
	}
	//nl := &NatsLogger{}
	//nsvr.SetLogger(nl, true, true)

	go nsvr.Start()
	if !nsvr.ReadyForConnections(5 * time.Second) {
		panic("start NATS ")
	}
	slog.Info("NATS started")

	var ssvr Server
	if defaults.sharServerImageUrl != "" {
		ssvr = inContainerSharServer(ssvr, defaults, natsPort)
	} else {
		ssvr = inProcessSharServer(sharConcurrency, apiAuth, authN, natsHost, natsPort)
	}

	slog.Info("Setup completed")
	return ssvr, nsvr, nil
}

func inContainerSharServer(ssvr Server, defaults *zenOpts, natsPort int) Server {
	ssvr = &ContainerisedServer{
		req: testcontainers.ContainerRequest{
			Image:        defaults.sharServerImageUrl,
			ExposedPorts: []string{"50000/TCP"},
			WaitingFor:   wait.ForExposedPort(),
			Env: map[string]string{
				"NATS_URL": fmt.Sprintf("nats://host.docker.internal:%d", natsPort),
			},
		},
	}
	ssvr.Listen("", 0)

	return ssvr
}

func inProcessSharServer(sharConcurrency int, apiAuth authz.APIFunc, authN authn.Check, natsHost string, natsPort int) *sharsvr.Server {
	options := []sharsvr.Option{
		sharsvr.EphemeralStorage(),
		sharsvr.PanicRecovery(false),
		sharsvr.Concurrency(sharConcurrency),
		sharsvr.WithNoHealthServer(),
	}
	if apiAuth != nil {
		options = append(options, sharsvr.WithApiAuthorizer(apiAuth))
	}
	if authN != nil {
		options = append(options, sharsvr.WithAuthentication(authN))
	}

	ssvr := sharsvr.New(options...)
	go ssvr.Listen(natsHost+":"+strconv.Itoa(natsPort), 0)
	for {
		if ssvr.Ready() {
			break
		}
		slog.Info("waiting for shar")
		time.Sleep(500 * time.Millisecond)
	}
	return ssvr
}

type Server interface {
	Shutdown()
	Listen(backend string, port int)
}

type ContainerisedServer struct {
	req       testcontainers.ContainerRequest
	container testcontainers.Container
}

func (cp *ContainerisedServer) Listen(_ string, _ int) {
	ctx := context.Background()
	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: cp.req,
		Started:          true,
	})

	if err != nil {
		panic(fmt.Sprintf("failed to start container for request: %+v", cp.req))
	}

	cp.container = container
}

func (cp *ContainerisedServer) Shutdown() {
	if cp.container != nil {
		ctx := context.Background()
		if err := cp.container.Terminate(ctx); err != nil {
			panic("failed to shutdown the container ")
		}
	}
}
