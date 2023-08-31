package integration_support

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"math/big"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/davecgh/go-spew/spew"
	"github.com/nats-io/nats.go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gitlab.com/shar-workflow/shar/common/authn"
	"gitlab.com/shar-workflow/shar/common/authz"
	"gitlab.com/shar-workflow/shar/common/logx"
	"gitlab.com/shar-workflow/shar/model"
	"gitlab.com/shar-workflow/shar/server/tools/tracer"
	server2 "gitlab.com/shar-workflow/shar/telemetry/server"
	zensvr "gitlab.com/shar-workflow/shar/zen-shar/server"
	"golang.org/x/exp/slog"
	"google.golang.org/protobuf/proto"
)

var errDirtyKV = errors.New("KV contains values when expected empty")

// Integration - the integration test support framework.
type Integration struct {
	testNatsServer zensvr.Server
	testSharServer zensvr.Server
	FinalVars      map[string]interface{}
	Test           *testing.T
	Mx             sync.Mutex
	Cooldown       time.Duration
	WithTelemetry  server2.Exporter
	testTelemetry  *server2.Server
	WithTrace      bool
	traceSub       *tracer.OpenTrace
	NatsURL        string // NatsURL is the default testing URL for the NATS host.
	NatsPort       int    // NatsPort is the default testing port for the NATS host.
	NatsHost       string // NatsHost is the default NATS host.
}

// Setup - sets up the test NATS and SHAR servers.
func (s *Integration) Setup(t *testing.T, authZFn authz.APIFunc, authNFn authn.Check) {
	s.NatsHost = "127.0.0.1"
	v, e := rand.Int(rand.Reader, big.NewInt(500))
	if e != nil {
		panic("no crypto:" + e.Error())
	}
	s.NatsPort = 4459 + int(v.Int64())
	s.NatsURL = fmt.Sprintf("nats://%s:%v", s.NatsHost, s.NatsPort)
	logx.SetDefault(slog.LevelDebug, true, "shar-Integration-tests")
	s.Cooldown = 10 * time.Second
	s.Test = t
	s.FinalVars = make(map[string]interface{})

	ss, ns, err := zensvr.GetServers(s.NatsHost, s.NatsPort, 10, authZFn, authNFn, zensvr.WithSharServerImageUrl(os.Getenv("SHAR_SERVER_IMAGE_URL")))
	if err != nil {
		panic(err)
	}
	if s.WithTrace {
		s.traceSub = tracer.Trace(s.NatsURL)
	}
	if s.WithTelemetry != nil {
		ctx := context.Background()
		n, err := nats.Connect(s.NatsURL)
		require.NoError(t, err)
		js, err := n.JetStream()
		require.NoError(t, err)
		s.testTelemetry = server2.New(ctx, js, s.WithTelemetry)
		err = s.testTelemetry.Listen()
		require.NoError(t, err)
	}

	s.testSharServer = ss
	s.testNatsServer = ns
	s.Test.Logf("Starting test support for " + s.Test.Name())
	s.Test.Logf("\033[1;36m%s\033[0m", "> Setup completed\n")
}

// AssertCleanKV - ensures SHAR has cleans up after itself, and there are no records left in the KV.
func (s *Integration) AssertCleanKV() {
	ctx, cancel := context.WithCancel(context.Background())
	errs := make(chan error, 1)
	var err error
	go func(ctx context.Context, cancel context.CancelFunc) {
		for {
			if ctx.Err() != nil {
				cancel()
				return
			}
			err = s.checkCleanKV()
			if err == nil {
				cancel()
				close(errs)
				return
			}
			if errors.Is(err, errDirtyKV) {
				time.Sleep(500 * time.Millisecond)
				continue
			}
			errs <- err
			cancel()
			return
		}
	}(ctx, cancel)

	select {
	case err2 := <-errs:
		cancel()
		assert.NoError(s.Test, err2, "KV not clean")
		return
	case <-time.After(s.Cooldown):
		cancel()
		if err != nil {
			assert.NoErrorf(s.Test, err, "KV not clean")
		}
		return
	}
}

func (s *Integration) checkCleanKV() error {
	js, err := s.GetJetstream()
	require.NoError(s.Test, err)

	for n := range js.KeyValueStores() {
		name := n.Bucket()
		kvs, err := js.KeyValue(name)
		require.NoError(s.Test, err)
		keys, err := kvs.Keys()
		if err != nil && errors.Is(err, nats.ErrNoKeysFound) {
			continue
		}
		require.NoError(s.Test, err)
		switch name {
		case "WORKFLOW_DEF",
			"WORKFLOW_NAME",
			"WORKFLOW_JOB",
			"WORKFLOW_INSTANCE",
			"WORKFLOW_PROCESS",
			"WORKFLOW_VERSION",
			"WORKFLOW_CLIENTTASK",
			"WORKFLOW_MSGID",
			"WORKFLOW_MSGNAME",
			"WORKFLOW_OWNERNAME",
			"WORKFLOW_OWNERID",
			"WORKFLOW_USERTASK",
			"WORKFLOW_MSGTYPES",
			"WORKFLOW_HISTORY",
			"WORKFLOW_TSKSPEC",
			"WORKFLOW_TSPECVER":
			//noop
		default:
			if len(keys) > 0 {
				sc := spew.ConfigState{
					Indent:                  "\t",
					MaxDepth:                2,
					DisableMethods:          true,
					DisablePointerMethods:   true,
					DisablePointerAddresses: true,
					DisableCapacities:       true,
					ContinueOnMethod:        false,
					SortKeys:                false,
					SpewKeys:                true,
				}

				for _, i := range keys {
					p, err := kvs.Get(i)
					if err == nil {
						str := &model.WorkflowState{}
						err := proto.Unmarshal(p.Value(), str)
						if err == nil {
							sc.Dump(str)
						} else {
							str := &model.MessageInstance{}
							err := proto.Unmarshal(p.Value(), str)
							if err == nil {
								sc.Dump(str)
							}
						}
					}
				}
				return fmt.Errorf("%d unexpected keys found in %s: %w", len(keys), name, errDirtyKV)
			}
		}
	}

	b, err := js.KeyValue("WORKFLOW_USERTASK")
	if err != nil && errors.Is(err, nats.ErrNoKeysFound) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("checkCleanKV failed to get usertasks: %w", err)
	}

	keys, err := b.Keys()
	if err != nil {
		if err == nats.ErrNoKeysFound {
			return nil
		}
		return fmt.Errorf("checkCleanKV failed to get user task keys: %w", err)
	}

	for _, k := range keys {
		bts, err := b.Get(k)
		if err != nil {
			return fmt.Errorf("checkCleanKV failed to get user task value: %w", err)
		}
		msg := &model.UserTasks{}
		err = proto.Unmarshal(bts.Value(), msg)
		if err != nil {
			return fmt.Errorf("checkCleanKV failed to unmarshal user task: %w", err)
		}
		if len(msg.Id) > 0 {
			return fmt.Errorf("unexpected UserTask %s found in WORKFLOW_USERTASK: %w", msg.Id, errDirtyKV)
		}
	}

	return nil
}

// Teardown - resposible for shutting down the integration test framework.
func (s *Integration) Teardown() {
	if s.WithTrace {
		s.traceSub.Close()
	}
	n, err := s.GetJetstream()
	require.NoError(s.Test, err)

	sub, err := n.PullSubscribe("WORKFLOW.>", "fin")
	require.NoError(s.Test, err)
	for {
		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
		_, err := sub.Fetch(1, nats.Context(ctx))
		if errors.Is(err, context.DeadlineExceeded) {
			cancel()
			break
		}
		if err != nil {
			cancel()
			s.Test.Fatal(err)
		}

		cancel()
	}
	s.Test.Log("TEARDOWN")
	s.testSharServer.Shutdown()
	s.testNatsServer.Shutdown()
	s.Test.Log("NATS shut down")
	s.Test.Logf("\033[1;36m%s\033[0m", "> Teardown completed")
	s.Test.Log("\n")
}

// GetJetstream - fetches the test framework jetstream server for making test calls.
//
//goland:noinspection GoUnnecessarilyExportedIdentifiers
func (s *Integration) GetJetstream() (nats.JetStreamContext, error) { //nolint:ireturn
	con, err := s.GetNats()
	if err != nil {
		return nil, fmt.Errorf("get NATS: %w", err)
	}
	js, err := con.JetStream()
	if err != nil {
		return nil, fmt.Errorf("obtain JetStream connection: %w", err)
	}
	return js, nil
}

// GetNats - fetches the test framework NATS server for making test calls.
//
//goland:noinspection GoUnnecessarilyExportedIdentifiers
func (s *Integration) GetNats() (*nats.Conn, error) {
	con, err := nats.Connect(s.NatsURL)
	if err != nil {
		return nil, fmt.Errorf("connect to NATS: %w", err)
	}
	return con, nil
}

// WaitForChan waits for a chan struct{} with a duration timeout.
func WaitForChan(t *testing.T, c chan struct{}, d time.Duration) {
	select {
	case <-c:
		return
	case <-time.After(d):
		assert.Fail(t, "timed out waiting for completion")
		return
	}
}
