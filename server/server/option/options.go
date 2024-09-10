package option

import (
	version2 "github.com/hashicorp/go-version"
	"github.com/nats-io/nats.go"
	"gitlab.com/shar-workflow/shar/common/authn"
	"gitlab.com/shar-workflow/shar/common/authz"
	"gitlab.com/shar-workflow/shar/common/telemetry"
	"log/slog"
)

// ServerOptions contains settings that control various aspects of shar operation and behaviour
type ServerOptions struct {
	PanicRecovery           bool
	AllowOrphanServiceTasks bool
	Concurrency             int
	ApiAuthorizer           authz.APIFunc
	ApiAuthenticator        authn.Check
	HealthServiceEnabled    bool
	SharVersion             *version2.Version
	NatsUrl                 string
	//conn                    *nats.Conn
	GrpcPort        int
	TelemetryConfig telemetry.Config
	ShowSplash      bool
	JetStreamDomain string
	NatsConnOptions []nats.Option
}

// Option represents a SHAR server option
type Option interface {
	Configure(serverOptions *ServerOptions)
}

// PanicRecovery enables or disables SHAR's ability to recover from server panics.
// This is on by default, and disabling it is not recommended for production use.
func PanicRecovery(enabled bool) panicOption { //nolint
	return panicOption{value: enabled}
}

type panicOption struct{ value bool }

func (o panicOption) Configure(serverOptions *ServerOptions) {
	serverOptions.PanicRecovery = o.value
}

// PreventOrphanServiceTasks enables or disables SHAR's validation of service task names againt existing workflows.
func PreventOrphanServiceTasks() orphanTaskOption { //nolint
	return orphanTaskOption{value: true}
}

type orphanTaskOption struct{ value bool }

func (o orphanTaskOption) Configure(serverOptions *ServerOptions) {
	serverOptions.AllowOrphanServiceTasks = o.value
}

// Concurrency specifies the number of threads for each of SHAR's queue listeneres.
func Concurrency(n int) concurrencyOption { //nolint
	return concurrencyOption{value: n}
}

type concurrencyOption struct{ value int }

func (o concurrencyOption) Configure(serverOptions *ServerOptions) {
	serverOptions.Concurrency = o.value
}

// WithApiAuthorizer specifies a handler function for API authorization.
func WithApiAuthorizer(authFn authz.APIFunc) apiAuthorizerOption { //nolint
	return apiAuthorizerOption{value: authFn}
}

type apiAuthorizerOption struct{ value authz.APIFunc }

func (o apiAuthorizerOption) Configure(serverOptions *ServerOptions) {
	slog.Warn("AuthZ set")
	serverOptions.ApiAuthorizer = o.value
}

// WithAuthentication specifies a handler function for API authorization.
func WithAuthentication(authFn authn.Check) authenticationOption { //nolint
	return authenticationOption{value: authFn}
}

type authenticationOption struct{ value authn.Check }

func (o authenticationOption) Configure(serverOptions *ServerOptions) {
	slog.Warn("AuthN set")
	serverOptions.ApiAuthenticator = o.value
}

// WithNoHealthServer specifies a handler function for API authorization.
func WithNoHealthServer() noHealthServerOption { //nolint
	return noHealthServerOption{}
}

type noHealthServerOption struct{}

func (o noHealthServerOption) Configure(serverOptions *ServerOptions) {
	serverOptions.HealthServiceEnabled = false
}

// WithSharVersion instructs SHAR to claim it is a specific version.
// This is highly inadvisable as datalos may occur.
func WithSharVersion(version *version2.Version) sharVersionOption { //nolint
	return sharVersionOption{version: version}
}

type sharVersionOption struct {
	version *version2.Version
}

func (o sharVersionOption) Configure(serverOptions *ServerOptions) {
	serverOptions.SharVersion = o.version
}

// NatsUrl specifies the nats URL to connect to
func NatsUrl(url string) natsUrlOption { //nolint
	return natsUrlOption{value: url}
}

type natsUrlOption struct{ value string }

func (o natsUrlOption) Configure(serverOptions *ServerOptions) {
	serverOptions.NatsUrl = o.value
}

// GrpcPort specifies the port healthcheck is listening on
func GrpcPort(port int) grpcPortOption { //nolint
	return grpcPortOption{value: port}
}

type grpcPortOption struct{ value int }

func (o grpcPortOption) Configure(serverOptions *ServerOptions) {
	serverOptions.GrpcPort = o.value
}

// WithTelemetryEndpoint specifies a handler function for API authorization.
func WithTelemetryEndpoint(endpoint string) telemetryEndpointOption { //nolint
	return telemetryEndpointOption{endpoint: endpoint}
}

type telemetryEndpointOption struct {
	endpoint string
}

func (o telemetryEndpointOption) Configure(serverOptions *ServerOptions) {
	serverOptions.TelemetryConfig = telemetry.Config{Enabled: o.endpoint != "", Endpoint: o.endpoint}
}

// WithShowSplash specifies whether to show a splash screen on the SHAR server startup.
// Enabling this option will make the splash screen be displayed.
func WithShowSplash() showSplashOption {
	return showSplashOption{showSplash: true}
}

type showSplashOption struct {
	showSplash bool
}

func (o showSplashOption) Configure(serverOptions *ServerOptions) {
	serverOptions.ShowSplash = o.showSplash
}

// WithJetStreamDomain specifies a handler function for API authorization.
func WithJetStreamDomain(jsDomain string) jetStreamDomainOption { //nolint
	return jetStreamDomainOption{value: jsDomain}
}

type jetStreamDomainOption struct{ value string }

func (o jetStreamDomainOption) Configure(serverOptions *ServerOptions) {
	serverOptions.JetStreamDomain = o.value
}
