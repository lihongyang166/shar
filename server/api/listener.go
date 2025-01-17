package api

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	version2 "github.com/hashicorp/go-version"
	"gitlab.com/shar-workflow/shar/common/ctxkey"
	"gitlab.com/shar-workflow/shar/common/header"
	"gitlab.com/shar-workflow/shar/common/logx"
	"gitlab.com/shar-workflow/shar/common/middleware"
	"gitlab.com/shar-workflow/shar/common/setup/upgrader"
	"gitlab.com/shar-workflow/shar/common/subj"
	"gitlab.com/shar-workflow/shar/common/telemetry"
	"gitlab.com/shar-workflow/shar/common/version"
	"gitlab.com/shar-workflow/shar/internal"
	"gitlab.com/shar-workflow/shar/server/server/option"
	"gitlab.com/shar-workflow/shar/server/services/natz"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/trace"
	"log/slog"
	"reflect"
	"runtime"
	"sync"

	"github.com/nats-io/nats.go"
	"gitlab.com/shar-workflow/shar/common"
	errors2 "gitlab.com/shar-workflow/shar/server/errors"
	"gitlab.com/shar-workflow/shar/server/messages"
	"google.golang.org/grpc/codes"
	"google.golang.org/protobuf/proto"
)

// Listener provides the mechanism via which api requests are received and responded to
type Listener struct {
	nc                   *natz.NatsConnConfiguration
	panicRecovery        bool
	subs                 *sync.Map
	tr                   trace.Tracer
	receiveApiMiddleware []middleware.Receive
	sendMiddleware       []middleware.Send
	endpointDefs         []endpointDef
}

// NewListener creates a new Listener
func NewListener(nc *natz.NatsConnConfiguration, options *option.ServerOptions) *Listener {

	l := &Listener{
		nc:                   nc,
		panicRecovery:        options.PanicRecovery,
		subs:                 &sync.Map{},
		tr:                   otel.GetTracerProvider().Tracer("shar", trace.WithInstrumentationVersion(version.Version)),
		receiveApiMiddleware: []middleware.Receive{telemetry.CtxWithTraceParentFromNatsMsgMiddleware(), telemetry.NatsMsgToCtxWithSpanMiddleware()},
		sendMiddleware:       []middleware.Send{telemetry.CtxSpanToNatsMsgMiddleware()},
		endpointDefs:         make([]endpointDef, 0, 50),
	}
	return l
}

var shutdownOnce sync.Once

// Shutdown gracefully shuts down the SHAR API server
func (s *Listener) Shutdown() {
	slog.Info("stopping shar api listener")
	shutdownOnce.Do(func() {
		s.subs.Range(func(key, _ any) bool {
			sub := key.(*nats.Subscription)
			if err := sub.Drain(); err != nil {
				slog.Error("drain subscription for "+sub.Subject, "error", err)
				return false
			}
			return true
		})
		slog.Info("shar api listener stopped")
	})
}

type endpointDef struct {
	name             string
	endpointSubject  string
	startListeningFn func() error
}

// RegisterEndpointFn define an endpoint with request/reply behaviour
func RegisterEndpointFn[Req proto.Message, Res proto.Message](l *Listener, name string, endpointSubject string, reqBuilderFn func() Req, endpointFn func(ctx context.Context, req Req) (Res, error)) {
	l.endpointDefs = append(l.endpointDefs, endpointDef{
		name:            name,
		endpointSubject: endpointSubject,
		startListeningFn: func() error {
			return listen(l.nc.Conn, l.panicRecovery, l.subs, endpointSubject, l.receiveApiMiddleware, reqBuilderFn(), endpointFn)
		},
	})
}

// RegisterEndpointStreamingFn define an endpoint with request/ streaming reply behaviour
func RegisterEndpointStreamingFn[Req proto.Message, Res proto.Message](l *Listener, name string, endpointSubject string, reqBuilderFn func() Req, endpointStreamingFn func(ctx context.Context, req Req, res chan<- Res, errs chan<- error)) {
	l.endpointDefs = append(l.endpointDefs, endpointDef{
		name:            name,
		endpointSubject: endpointSubject,
		startListeningFn: func() error {
			return ListenReturnStream(l.nc.Conn, l.panicRecovery, l.subs, endpointSubject, l.receiveApiMiddleware, reqBuilderFn(), endpointStreamingFn)
		},
	})
}

// StartListening iterates over the endpoint definitions and starts listening for requests to them
func (s *Listener) StartListening() error {
	for _, ep := range s.endpointDefs {
		if err := ep.startListeningFn(); err != nil {
			return fmt.Errorf("%s: %w", ep.name, err)
		}
	}

	slog.Info("listener started")
	return nil
}

// ListenReturnStream is a function that sets up a NATS subscription to handle streaming reply messages.
// It executes the provided function to process the request and send the response messages.
// The function runs in a separate goroutine that continuously listens for return messages and error messages, and publishes them to the reply inbox.
// the function exits when an error or cancellation occurs.
func ListenReturnStream[T proto.Message, U proto.Message](con common.NatsConn, panicRecovery bool, subList *sync.Map, subject string, receiveAPIMiddleware []middleware.Receive, req T, fn func(ctx context.Context, req T, res chan<- U, errs chan<- error)) error {
	sub, err := common.StreamingReplyServer(con, subject, func(msg *nats.Msg, retMsgs chan *nats.Msg, retErrs chan error) {
		if msg.Subject != messages.APIGetVersionInfo {
			callerVersion, err := version2.NewVersion(msg.Header.Get(header.NatsCompatHeader))
			if err != nil {
				retErrs <- errors.New(string(apiError(codes.PermissionDenied, "version: client version invalid")))
				return
			} else {
				if ok, ver := upgrader.IsCompatible(callerVersion); !ok {
					retErrs <- errors.New(string(apiError(codes.PermissionDenied, "version: client version >= "+ver.String()+" required")))
					return
				}
			}
		}
		ctx, log := logx.NatsMessageLoggingEntrypoint(context.Background(), "server", msg.Header)
		ctx = subj.SetNS(ctx, msg.Header.Get(header.SharNamespace))
		for _, i := range receiveAPIMiddleware {
			var err error
			ctx, err = i(ctx, common.NewNatsMsgWrapper(msg))
			if err != nil {
				retErrs <- errors.New(string(apiError(codes.Internal, fmt.Sprintf("receive middleware %s: %s", reflect.TypeOf(i), err.Error()))))
				return
			}
		}
		ctx, span := telemetry.StartApiSpan(ctx, "shar", msg.Subject)
		if err := callAPIReturnStream(ctx, panicRecovery, req, msg, retMsgs, retErrs, fn); err != nil {
			log.Error("API call for "+subject+" failed", "error", err)
		}
		span.End()
	})
	if err != nil {
		return fmt.Errorf("streaming subscribe to %s: %w", subject, err)
	}
	subList.Store(sub, struct{}{})
	return nil
}

func callAPIReturnStream[T proto.Message, U proto.Message](ctx context.Context, panicRecovery bool, container T, msg *nats.Msg, res chan<- *nats.Msg, errs chan<- error, fn func(ctx context.Context, req T, res chan<- U, errs chan<- error)) error {
	if panicRecovery {
		defer recoverAPIpanic(msg)
	}
	if err := proto.Unmarshal(msg.Data, container); err != nil {
		errorResponse(msg, codes.InvalidArgument, err.Error())
		return fmt.Errorf("unmarshal message data during callAPI: %w", err)
	}
	ctx, err := header.FromMsgHeaderToCtx(ctx, msg.Header)
	if err != nil {
		return errors2.ErrWorkflowFatal{Err: fmt.Errorf("decode context value from NATS message for API call: %w", err)}
	}
	ctx = context.WithValue(ctx, ctxkey.APIFunc, msg.Subject)
	iRes := make(chan U)
	iErrs := make(chan error, 1)
	go func() {
		fn(ctx, container, iRes, iErrs)
		close(iErrs)
	}()
	for {
		select {
		case e := <-iErrs:
			if e != nil {
				svrErr := errors.New(string(apiError(codes.Internal, e.Error())))
				errs <- svrErr
			}
			return e
		case r := <-iRes:
			b, err := proto.Marshal(r)
			if err != nil {
				//TODO: DEAL WITH WORKFLOW FATAL
				return fmt.Errorf("marshal streaming result: %w", err)
			}
			retMsg := nats.NewMsg("return")
			retMsg.Data = b
			res <- retMsg
		}
	}
}

func listen[T proto.Message, U proto.Message](con common.NatsConn, panicRecovery bool, subList *sync.Map, subject string, receiveApiMiddleware []middleware.Receive, req T, fn func(ctx context.Context, req T) (U, error)) error {
	sub, err := con.QueueSubscribe(subject, subject, func(msg *nats.Msg) {
		if msg.Subject != messages.APIGetVersionInfo {
			callerVersion, err := version2.NewVersion(msg.Header.Get(header.NatsCompatHeader))
			if err != nil {
				errorResponse(msg, codes.PermissionDenied, "version: client version invalid")
				return
			} else {
				if ok, ver := upgrader.IsCompatible(callerVersion); !ok {
					errorResponse(msg, codes.PermissionDenied, "version: client version >= "+ver.String()+" required")
					return
				}
			}
		}
		ctx, log := logx.NatsMessageLoggingEntrypoint(context.Background(), "server", msg.Header)
		ctx = subj.SetNS(ctx, msg.Header.Get(header.SharNamespace))
		for _, i := range receiveApiMiddleware {
			var err error
			ctx, err = i(ctx, common.NewNatsMsgWrapper(msg))
			if err != nil {
				errorResponse(msg, codes.Internal, fmt.Sprintf("receive middleware %s: %s", reflect.TypeOf(i), err.Error()))
				return
			}
		}
		ctx, span := telemetry.StartApiSpan(ctx, "shar", msg.Subject)
		if err := callAPI(ctx, panicRecovery, req, msg, fn); err != nil {
			log.Error("API call for "+subject+" failed", "error", err)
		}
		span.End()
	})
	if err != nil {
		return fmt.Errorf("subscribe to %s: %w", subject, err)
	}
	subList.Store(sub, struct{}{})
	return nil
}

func callAPI[T proto.Message, U proto.Message](ctx context.Context, panicRecovery bool, container T, msg *nats.Msg, fn func(ctx context.Context, req T) (U, error)) error {
	if panicRecovery {
		defer recoverAPIpanic(msg)
	}
	if err := proto.Unmarshal(msg.Data, container); err != nil {
		errorResponse(msg, codes.InvalidArgument, err.Error())
		return fmt.Errorf("unmarshal message data during callAPI: %w", err)
	}
	ctx, err := header.FromMsgHeaderToCtx(ctx, msg.Header)
	if err != nil {
		return errors2.ErrWorkflowFatal{Err: fmt.Errorf("decode context value from NATS message for API call: %w", err)}
	}
	ctx = context.WithValue(ctx, ctxkey.APIFunc, msg.Subject)
	resMsg, err := fn(ctx, container)
	if err != nil {
		c := codes.Unknown
		if errors2.IsWorkflowFatal(err) {
			c = codes.Internal
		}
		errorResponse(msg, c, err.Error())
		return fmt.Errorf("API call: %w", err)
	}
	res, err := proto.Marshal(resMsg)
	if err != nil {
		errorResponse(msg, codes.InvalidArgument, err.Error())
		return fmt.Errorf("unmarshal API response: %w", err)
	}
	if err := msg.Respond(res); err != nil {
		errorResponse(msg, codes.FailedPrecondition, err.Error())
		return fmt.Errorf("API response: %w", err)
	}
	return nil
}

func recoverAPIpanic(msg *nats.Msg) {
	if r := recover(); r != nil {
		buf := make([]byte, 16384)
		runtime.Stack(buf, false)
		stack := buf[:bytes.IndexByte(buf, 0)]
		fmt.Println(stack)
		errorResponse(msg, codes.Internal, r)
		slog.Info("recovered from ", r)
	}
}

func errorResponse(m *nats.Msg, code codes.Code, msg any) {
	if err := m.Respond(apiError(code, msg)); err != nil {
		slog.Error("send error response: "+string(apiError(codes.Internal, msg)), "error", err)
	}
}

func apiError(code codes.Code, msg any) []byte {
	err := fmt.Sprintf("%s%d%s%+v", internal.ErrorPrefix, code, internal.ErrorSeparator, msg)
	return []byte(err)
}
