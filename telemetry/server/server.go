package server

import (
	"bytes"
	"context"
	_ "embed"
	"encoding/hex"
	"errors"
	"fmt"
	"github.com/nats-io/nats.go/jetstream"
	model2 "gitlab.com/shar-workflow/shar/internal/model"
	"log/slog"
	"strings"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/segmentio/ksuid"
	"gitlab.com/shar-workflow/shar/common"
	"gitlab.com/shar-workflow/shar/common/logx"
	"gitlab.com/shar-workflow/shar/common/middleware"
	"gitlab.com/shar-workflow/shar/common/namespace"
	"gitlab.com/shar-workflow/shar/common/setup"
	"gitlab.com/shar-workflow/shar/model"
	"gitlab.com/shar-workflow/shar/server/errors/keys"
	"gitlab.com/shar-workflow/shar/server/messages"
	"gitlab.com/shar-workflow/shar/telemetry/config"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/sdk/instrumentation"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	tracesdk "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.10.0"
	semconv2 "go.opentelemetry.io/otel/semconv/v1.12.0"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/protobuf/proto"
)

const (
	service     = "shar"
	environment = "production"
	id          = 1
)

// NatsConfig holds the current configuration of the SHAR Telemetry Server
//
//go:embed nats-config.yaml
var NatsConfig string

var startActions = []string{
	messages.StateExecutionExecute, messages.StateProcessExecute, messages.StateTraversalExecute, messages.StateActivityExecute,
	messages.StateJobExecuteServiceTask, messages.StateJobExecuteUserTask, messages.StateJobExecuteManualTask, messages.StateJobExecuteSendMessage,
}

var endActions = []string{
	messages.StateTraversalComplete, messages.StateActivityComplete, messages.StateActivityAbort, messages.StateJobCompleteServiceTask,
	messages.StateExecutionComplete, messages.StateProcessTerminated, messages.StateJobAbortServiceTask, messages.StateJobCompleteUserTask,
	messages.StateJobCompleteManualTask, messages.StateJobCompleteSendMessage,
}

var endStartActionMapping = map[string]string{
	messages.StateTraversalComplete:      messages.StateTraversalExecute,
	messages.StateActivityComplete:       messages.StateActivityExecute,
	messages.StateActivityAbort:          messages.StateActivityExecute,
	messages.StateJobCompleteServiceTask: messages.StateJobExecuteServiceTask,
	messages.StateExecutionComplete:      messages.StateExecutionExecute,
	messages.StateProcessTerminated:      messages.StateProcessExecute,
	messages.StateJobAbortServiceTask:    messages.StateJobExecuteServiceTask,
	messages.StateJobCompleteUserTask:    messages.StateJobExecuteUserTask,
	messages.StateJobCompleteManualTask:  messages.StateJobExecuteManualTask,
	messages.StateJobCompleteSendMessage: messages.StateJobExecuteSendMessage,
}

// Server is the shar server type responsible for hosting the telemetry server.
type Server struct {
	js     jetstream.JetStream
	spanKV jetstream.KeyValue
	res    *resource.Resource
	exp    Exporter

	wfStateCounter       metric.Int64Counter
	wfStateUpDownCounter metric.Int64UpDownCounter
}

// New creates a new telemetry server.
func New(ctx context.Context, nc *nats.Conn, js jetstream.JetStream, storageType jetstream.StorageType, exp Exporter) *Server {
	// Define our resource
	res := resource.NewWithAttributes(
		semconv.SchemaURL,
		semconv.ServiceNameKey.String(service),
		attribute.String("environment", environment),
		attribute.Int64("ID", id),
	)

	if err := setup.Nats(ctx, nc, js, storageType, NatsConfig, true, namespace.Default); err != nil {
		panic(err)
	}

	wfStateCounter, err := otel.GetMeterProvider().
		Meter(
			"instrumentation/server",
			metric.WithInstrumentationVersion("0.0.1"),
		).
		Int64Counter(
			"workflow_state",
			metric.WithDescription("how many workflow state messages have been received, tagged by subject"),
		)

	if err != nil {
		slog.Error("err getting meter provider meter counter", "err", err.Error())
	}

	wfStateUpDownCounter, err := otel.GetMeterProvider().
		Meter(
			"instrumentation/server",
			metric.WithInstrumentationVersion("0.0.1"),
		).
		Int64UpDownCounter(
			"workflow_state_up_down",
			metric.WithDescription("how many workflow state messages are active, tagged by action"),
		)
	if err != nil {
		slog.Error("err getting meter provider meter up down counter", "err", err.Error())
	}

	return &Server{
		js:                   js,
		res:                  res,
		exp:                  exp,
		wfStateCounter:       wfStateCounter,
		wfStateUpDownCounter: wfStateUpDownCounter,
	}
}

// Listen starts the telemetry server.
func (s *Server) Listen() error {
	ctx := context.Background()
	closer := make(chan struct{})

	kv, err := s.js.KeyValue(ctx, namespace.PrefixWith(namespace.Default, messages.KvTracking))
	if err != nil {
		return fmt.Errorf("listen failed to attach to tracking key value database: %w", err)
	}
	s.spanKV = kv
	err = common.Process(ctx, s.js, "WORKFLOW_TELEMETRY", "telemetry", closer, "WORKFLOW.*.State.>", "Tracing", 1, []middleware.Receive{}, s.workflowTrace, nil)
	if err != nil {
		return fmt.Errorf("listen failed to start telemetry handler: %w", err)
	}
	return nil
}

var empty8 = [8]byte{}

func (s *Server) workflowTrace(ctx context.Context, log *slog.Logger, msg jetstream.Msg) (bool, error) {
	state, done, err2 := s.decodeState(ctx, msg.Data())
	if done {
		return done, err2
	}

	switch {
	case strings.HasSuffix(msg.Subject(), messages.StateExecutionExecute):
		s.incrementActionCounter(ctx, messages.StateExecutionExecute)
		s.changeActionUpDownCounter(ctx, 1, messages.StateExecutionExecute)
		// TODO: we should add a trace ID field and change this in future
		// in case we are passed a trace ID that is used instead of this
		ks, err := ksuid.Parse(state.ExecutionId)
		if err != nil {
			slog.Error("error parsing trace ID: %w", "error", err)
		}
		slog.Debug(
			"execution execute",
			slog.String("hexTraceID", hex.EncodeToString(ks.Payload())),
			slog.String("executionId", state.ExecutionId),
			slog.String("workflowId", state.WorkflowId),
		)

		if err := s.saveSpan(ctx, "Execution Start", state, state); err != nil {
			return true, nil
		}
	case strings.HasSuffix(msg.Subject(), messages.StateProcessExecute):
		s.incrementActionCounter(ctx, messages.StateProcessExecute)
		s.changeActionUpDownCounter(ctx, 1, messages.StateProcessExecute)

		if err := s.saveSpan(ctx, "Process Start", state, state); err != nil {
			return true, nil
		}
	case strings.HasSuffix(msg.Subject(), messages.StateTraversalExecute):
	case strings.HasSuffix(msg.Subject(), messages.StateActivityExecute):
		if err := s.spanStart(ctx, state, msg.Subject()); err != nil {
			return true, nil
		}
	case strings.Contains(msg.Subject(), messages.StateJobExecuteServiceTask),
		strings.HasSuffix(msg.Subject(), messages.StateJobExecuteUserTask),
		strings.HasSuffix(msg.Subject(), messages.StateJobExecuteManualTask),
		strings.Contains(msg.Subject(), messages.StateJobExecuteSendMessage):
		if err := s.spanStart(ctx, state, msg.Subject()); err != nil {
			return true, nil
		}
	case strings.HasSuffix(msg.Subject(), messages.StateTraversalComplete):
	case strings.HasSuffix(msg.Subject(), messages.StateActivityComplete),
		strings.HasSuffix(msg.Subject(), messages.StateActivityAbort):
		if err := s.spanEnd(ctx, "Activity: "+state.ElementId, state, msg.Subject()); err != nil {
			var escape *AbandonOpError
			if errors.As(err, &escape) {
				log.Error("saving Activity.Complete operation abandoned", "error", err,
					slog.String(keys.ExecutionID, state.ExecutionId),
					slog.String(keys.TrackingID, common.TrackingID(state.Id).ID()),
					slog.String(keys.ParentTrackingID, common.TrackingID(state.Id).ParentID()),
					slog.String(keys.ElementType, state.ElementType),
				)
				return true, err
			}
			return true, nil
		}
	case strings.Contains(msg.Subject(), messages.StateJobCompleteServiceTask),
		strings.Contains(msg.Subject(), messages.StateJobAbortServiceTask),
		strings.Contains(msg.Subject(), messages.StateJobCompleteUserTask),
		strings.Contains(msg.Subject(), messages.StateJobCompleteManualTask),
		strings.Contains(msg.Subject(), messages.StateJobCompleteSendMessage):

		if err := s.spanEnd(ctx, "Job: "+state.ElementType, state, msg.Subject()); err != nil {
			var escape *AbandonOpError
			if errors.As(err, &escape) {
				log.Error("span end", "error", err,
					slog.String(keys.ExecutionID, state.ExecutionId),
					slog.String(keys.TrackingID, common.TrackingID(state.Id).ID()),
					slog.String(keys.ParentTrackingID, common.TrackingID(state.Id).ParentID()),
					slog.String(keys.ElementType, state.ElementType),
					slog.String("msg.Subject", msg.Subject()),
				)
				return true, err
			}
			return true, nil
		}
	case strings.Contains(msg.Subject(), messages.StateExecutionComplete),
		strings.Contains(msg.Subject(), messages.StateProcessTerminated):

		endAction := actionFrom(msg.Subject(), endActions)
		s.changeActionUpDownCounter(ctx, -1, startActionFor(endAction))
	case strings.Contains(msg.Subject(), messages.StateJobCompleteSendMessage):
	case strings.Contains(msg.Subject(), messages.StateLog):

	// case strings.HasSuffix(msg.Subject(), ".State.Execution.Complete"):
	// case strings.HasSuffix(msg.Subject(), ".State.Execution.Terminated"):
	default:

	}
	return true, nil
}

func (s *Server) incrementActionCounter(ctx context.Context, action string) {
	s.wfStateCounter.Add(
		ctx,
		1,
		// labels/tags
		metric.WithAttributes(attribute.String("wf_action", action)),
	)
}

func (s *Server) changeActionUpDownCounter(ctx context.Context, incr int64, action string) {
	s.wfStateUpDownCounter.Add(
		ctx,
		incr,
		// labels/tags
		metric.WithAttributes(attribute.String("wf_action", action)),
	)
}

func (s *Server) decodeState(ctx context.Context, data []byte) (*model.WorkflowState, bool, error) {
	log := logx.FromContext(ctx)
	state := &model.WorkflowState{}
	err := proto.Unmarshal(data, state)
	if err != nil {
		log.Error("unmarshal span", "error", err)
		return &model.WorkflowState{}, true, abandon(err)
	}

	tid := common.KSuidTo64bit(common.TrackingID(state.Id).ID())

	if bytes.Equal(tid[:], empty8[:]) {
		return &model.WorkflowState{}, true, nil
	}
	return state, false, nil
}

func actionFrom(subject string, startOrEndActions []string) string {
	for _, action := range startOrEndActions {
		if strings.Contains(subject, action) {
			return action
		}
	}
	return "UNKNOWN_ACTION"
}

func (s *Server) spanStart(ctx context.Context, state *model.WorkflowState, subject string) error {
	action := actionFrom(subject, startActions)
	s.incrementActionCounter(ctx, action)
	s.changeActionUpDownCounter(ctx, 1, action)

	err := common.SaveObj(ctx, s.spanKV, common.TrackingID(state.Id).ID(), state)
	if err != nil {
		return fmt.Errorf("span-start failed fo save object: %w", err)
	}
	return nil
}

func startActionFor(endAction string) string {
	startAction, ok := endStartActionMapping[endAction]
	if ok {
		return startAction
	}
	return "UNKNOWN_START_ACTION"
}

func (s *Server) spanEnd(ctx context.Context, name string, state *model.WorkflowState, subject string) error {
	endAction := actionFrom(subject, endActions)
	s.changeActionUpDownCounter(ctx, -1, startActionFor(endAction))

	log := logx.FromContext(ctx)
	oldState := model.WorkflowState{}
	if err := common.LoadObj(ctx, s.spanKV, common.TrackingID(state.Id).ID(), &oldState); err != nil {
		log.Error("load span state:", "error", err, slog.String(keys.TrackingID, common.TrackingID(state.Id).ID()))
		return abandon(err)
	}
	state.ExecutionId = oldState.ExecutionId
	state.Id = oldState.Id
	state.WorkflowId = oldState.WorkflowId
	state.ElementId = oldState.ElementId
	state.ElementName = oldState.ElementName
	state.Execute = oldState.Execute
	state.Condition = oldState.Condition
	state.ElementType = oldState.ElementType
	state.State = oldState.State
	if err := s.saveSpan(ctx, name, &oldState, state); err != nil {
		log.Error("record span:", "error", err, slog.String(keys.TrackingID, common.TrackingID(state.Id).ID()))
		return fmt.Errorf("save span failed: %w", err)
	}
	return nil
}

func (s *Server) saveSpan(ctx context.Context, name string, oldState *model.WorkflowState, newState *model.WorkflowState) error {
	log := logx.FromContext(ctx)
	traceID := common.KSuidTo128bit(oldState.ExecutionId)

	spanID := common.KSuidTo64bit(common.TrackingID(oldState.Id).ID())
	parentID := common.KSuidTo64bit(common.TrackingID(oldState.Id).ParentID())
	parentSpan := trace.SpanContext{}
	if len(common.TrackingID(oldState.Id).ParentID()) > 0 {
		parentSpan = trace.NewSpanContext(trace.SpanContextConfig{
			TraceID: traceID,
			SpanID:  parentID,
		})
	}
	pid := common.TrackingID(oldState.Id).ParentID()
	id := common.TrackingID(oldState.Id).ID()
	st := oldState.State.String()
	at := map[string]*string{
		keys.ElementID:   &oldState.ElementId,
		keys.ElementName: &oldState.ElementName,
		keys.ElementType: &oldState.ElementType,
		keys.WorkflowID:  &oldState.WorkflowId,
		keys.ExecutionID: &oldState.ExecutionId,
		keys.Condition:   oldState.Condition,
		keys.Execute:     oldState.Execute,
		keys.State:       &st,
		"trackingId":     &id,
		"parentTrId":     &pid,
	}

	kv := model2.NewServerVars()
	if err := kv.Decode(ctx, newState.Vars); err != nil {
		return abandon(err)
	}

	for k, v := range kv.Vals {
		val := fmt.Sprintf("%+v", v)
		at["var."+k] = &val
	}

	attrs := buildAttrs(at)
	err := s.exp.ExportSpans(
		ctx,
		[]tracesdk.ReadOnlySpan{
			&sharSpan{
				ReadOnlySpan: nil,
				SpanName:     name,
				SpanCtx: trace.NewSpanContext(trace.SpanContextConfig{
					TraceID:    traceID,
					SpanID:     spanID,
					TraceFlags: 0,
					TraceState: trace.TraceState{},
					Remote:     false,
				}),
				SpanParent: parentSpan,
				Kind:       0,
				Start:      time.Unix(0, oldState.UnixTimeNano),
				End:        time.Unix(0, newState.UnixTimeNano),
				Attrs:      attrs,
				SpanLinks:  []tracesdk.Link{},
				SpanEvents: []tracesdk.Event{},
				SpanStatus: tracesdk.Status{
					Code: codes.Ok,
				},
				InstrumentationLib: instrumentation.Scope{
					Name:    "TEST",
					Version: "0.1",
				},
				SpanResource: s.res,
				ChildCount:   0,
			},
		},
	)
	if err != nil {
		return fmt.Errorf("export spans failed: %w", err)
	}
	err = s.spanKV.Delete(ctx, common.TrackingID(oldState.Id).ID())
	if err != nil {
		id := common.TrackingID(oldState.Id).ID()
		log.Warn("delete the cached span", "error", err, slog.String(keys.TrackingID, id))
	}
	return nil
}

func buildAttrs(m map[string]*string) []attribute.KeyValue {
	ret := make([]attribute.KeyValue, 0, len(m))
	for k, v := range m {
		if v != nil && *v != "" {
			ret = append(ret, attribute.String(k, *v))
		}
	}
	return ret
}

// SetupMetrics initialises metrics
func SetupMetrics(ctx context.Context, cfg *config.Settings, serviceName string) (*sdkmetric.MeterProvider, error) {
	//c, err := getTls()
	//if err != nil {
	//	return nil, err
	//}

	opts := []otlpmetrichttp.Option{otlpmetrichttp.WithEndpoint(cfg.OTLPEndpoint)}
	if !cfg.OTLPEndpointIsSecure {
		opts = append(opts, otlpmetrichttp.WithInsecure())
	}

	exporter, err := otlpmetrichttp.New(
		ctx,
		opts...,
	//otlpmetricgrpc.WithTLSCredentials(
	//	// mutual tls.
	//	credentials.NewTLS(c),
	//),
	)
	if err != nil {
		return nil, fmt.Errorf("failed creation of metrics exporter: %w", err)
	}

	// labels/tags/resources that are common to all metrics.
	res := resource.NewWithAttributes(
		semconv2.SchemaURL,
		semconv2.ServiceNameKey.String(serviceName),
		attribute.String("app", "shar-telemetry"),
	)

	mp := sdkmetric.NewMeterProvider(
		sdkmetric.WithResource(res),
		sdkmetric.WithReader(
			// collects and exports metric data every N seconds.
			sdkmetric.NewPeriodicReader(exporter, sdkmetric.WithInterval(10*time.Second)),
		),
	)

	otel.SetMeterProvider(mp)

	return mp, nil
}
