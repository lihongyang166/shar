package server

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"time"

	_ "embed"
	"github.com/docker/go-connections/nat"
	"github.com/nats-io/nats-server/v2/server"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
	"gitlab.com/shar-workflow/shar/common/authn"
	"gitlab.com/shar-workflow/shar/common/authz"
	version2 "gitlab.com/shar-workflow/shar/common/version"
	sharsvr "gitlab.com/shar-workflow/shar/server/server"
	"golang.org/x/exp/slog"
)

const (
	dockerHostName = "host.docker.internal"
)

type zenOpts struct {
	sharVersion        string
	sharServerImageUrl string
	natsServerImageUrl string
}

// ZenSharOptionApplyFn represents a SHAR Zen Server configuration function
type ZenSharOptionApplyFn func(cfg *zenOpts)

// WithSharVersion artificially sets the reported server version.
func WithSharVersion(ver string) ZenSharOptionApplyFn {
	return func(cfg *zenOpts) {
		cfg.sharVersion = ver
	}
}

// WithSharServerImageUrl will make zen-shar start shar server in a container from the specificed image URL
func WithSharServerImageUrl(imageUrl string) ZenSharOptionApplyFn {
	return func(cfg *zenOpts) {
		cfg.sharServerImageUrl = imageUrl
	}
}

// WithNatsServerImageUrl will make zen-shar start nats server in a container from the specificed image URL
func WithNatsServerImageUrl(imageUrl string) ZenSharOptionApplyFn {
	return func(cfg *zenOpts) {
		cfg.natsServerImageUrl = imageUrl
	}
}

// GetServers returns a test NATS and SHAR server.
//

//go:embed test-nats-config.conf
var natsConfig []byte

//nolint:ireturn
func GetServers(natsHost string, natsPort int, sharConcurrency int, apiAuth authz.APIFunc, authN authn.Check, option ...ZenSharOptionApplyFn) (Server, Server, error) {

	defaults := &zenOpts{sharVersion: version2.Version}
	for _, i := range option {
		i(defaults)
	}

	var nsvr Server
	var nHost string
	var nPort int

	natsConfigPath := fmt.Sprintf("%s/test-nats-config.conf", os.Getenv("TMPDIR"))
	err := os.WriteFile(natsConfigPath, natsConfig, 0644)
	if err != nil {
		panic(fmt.Errorf("failed writing nats config %w", err))
	}
	if defaults.natsServerImageUrl != "" {
		defaultNatsContainerPort := "4222"
		nsvr := inContainerNatsServer(defaults.natsServerImageUrl, defaultNatsContainerPort)
		nHost = dockerHostName
		nPort = nsvr.exposedToHostPorts[defaultNatsContainerPort]
	} else {
		nsvr = inProcessNatsServer(natsConfigPath, natsHost, natsPort)
		nHost = natsHost
		nPort = natsPort
	}

	var ssvr Server
	if defaults.sharServerImageUrl != "" {
		// TODO a combo of in container shar/in process nats will lead to problems as
		//in container shar will persist nats data to disk - an issue if subsequent tests
		//try to run against a nats instance that has previously written state to disk
		//(as nats/nats client does not permit this)???

		//should we restrict things to all in process OR all in container testing???
		//this is probably the simplest thing...
		ssvr = inContainerSharServer(defaults.sharServerImageUrl, dockerHostName, nPort)
	} else {
		ssvr = inProcessSharServer(sharConcurrency, apiAuth, authN, nHost, nPort)
	}

	slog.Info("Setup completed")
	return ssvr, nsvr, nil
}

func inProcessNatsServer(natsConfig string, natsHost string, natsPort int) *NatsServer {
	nHost := natsHost
	nPort := natsPort

	n := &NatsServer{natsConfig: natsConfig}
	n.Listen(nHost, nPort)
	return n
}

func inContainerSharServer(sharServerImageUrl string, natsHost string, natsPort int) *containerisedServer {
	ssvr := newContainerisedServer(testcontainers.ContainerRequest{
		Image:        sharServerImageUrl,
		ExposedPorts: []string{"50000/tcp"},
		WaitingFor:   wait.ForLog("shar api listener started"),
		Env: map[string]string{
			"NATS_URL": fmt.Sprintf("nats://%s:%d", natsHost, natsPort),
		}})

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

func inContainerNatsServer(natsServerImageUrl string, containerNatsPort string) *containerisedServer {
	ssvr := newContainerisedServer(testcontainers.ContainerRequest{
		Image:        natsServerImageUrl,
		ExposedPorts: []string{containerNatsPort},
		WaitingFor:   wait.ForLog("Listening for client connections").WithStartupTimeout(10 * time.Second),
	})

	ssvr.Listen("", 0)

	return ssvr
}

// Server is a general interface representing either an inprocess or in container Shar server
type Server interface {
	Shutdown()
	Listen(host string, port int)
}

// NatsServer is a wrapper around the nats lib server so that its lifecycle can be defined
// in terms of the Server interface needed by integration tests
type NatsServer struct {
	nsvr       *server.Server
	natsConfig string
}

// Listen starts an in process nats server
func (natserver *NatsServer) Listen(natsHost string, natsPort int) {
	//wd, err := os.Getwd()
	//if err != nil {
	//	return nil, nil, fmt.Errorf("failed to get working directory: %w", err)
	//}

	natsOptions, err := server.ProcessConfigFile(natserver.natsConfig)
	if err != nil {
		panic(fmt.Errorf("failed to load conf with err %w", err))
	}
	natsOptions.Host = natsHost
	natsOptions.Port = natsPort

	nsvr, err := server.NewServer(natsOptions)

	if err != nil {
		// return nil, nil, fmt.Errorf("create a new server instance: %w", err)
		panic(fmt.Errorf("create a new server instance: %w", err))
	}
	//nl := &NatsLogger{}
	//nsvr.SetLogger(nl, true, true)

	go nsvr.Start()
	if !nsvr.ReadyForConnections(5 * time.Second) {
		panic("start NATS ")
	}
	slog.Info("NATS started")

	natserver.nsvr = nsvr
}

// Shutdown shutsdown an in process nats server
func (natserver *NatsServer) Shutdown() {
	natserver.nsvr.Shutdown()
	natserver.nsvr.WaitForShutdown()
}

func newContainerisedServer(req testcontainers.ContainerRequest) *containerisedServer {
	svr := &containerisedServer{
		req:                req,
		exposedToHostPorts: make(map[string]int),
	}
	return svr
}

// containerisedServer is a wrapper to the test containers test library allowing you to start or shut
// any Server you wish to startup/shutdown in a container
type containerisedServer struct {
	req                testcontainers.ContainerRequest
	container          testcontainers.Container
	exposedToHostPorts map[string]int
}

// Listen will startup the server in a container
func (cp *containerisedServer) Listen(_ string, _ int) {
	ctx := context.Background()
	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: cp.req,
		Started:          true,
	})

	if err != nil {
		slog.Error(fmt.Sprintf("failed to start container for request: %+v", cp.req))
		panic(err)
	}

	cp.container = container

	if len(cp.req.ExposedPorts) > 0 {
		for _, exposedPort := range cp.req.ExposedPorts {
			natPort, err := container.MappedPort(ctx, nat.Port(exposedPort))
			if err != nil {
				panic(err)
			}
			cp.exposedToHostPorts[exposedPort] = natPort.Int()
		}
	}
}

// Shutdown will hutdown the containerised shar server
func (cp *containerisedServer) Shutdown() {
	if cp.container != nil {
		ctx := context.Background()
		if err := cp.container.Terminate(ctx); err != nil {
			panic("failed to shutdown the container ")
		}
	}
}
