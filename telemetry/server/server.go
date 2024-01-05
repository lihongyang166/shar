package server

import (
	"bytes"
	"context"
	_ "embed"
	"errors"
	"fmt"
	"github.com/nats-io/nats.go"
	"gitlab.com/shar-workflow/shar/common"
	"gitlab.com/shar-workflow/shar/common/logx"
	"gitlab.com/shar-workflow/shar/common/setup"
	"gitlab.com/shar-workflow/shar/model"
	"gitlab.com/shar-workflow/shar/server/errors/keys"
	"gitlab.com/shar-workflow/shar/server/messages"
	"gitlab.com/shar-workflow/shar/server/vars"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	//"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc"
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
	"log/slog"
	"strings"
	"time"
)

const (
	service                     = "shar"
	environment                 = "production"
	id                          = 1
	workflowTelemetryStreamName = "WORKFLOW-TELEMETRY"

	StateExecutionExecute      = ".State.Execution.Execute"
	StateProcessExecute        = ".State.Process.Execute"
	StateTraversalExecute      = ".State.Traversal.Execute"
	StateActivityExecute       = ".State.Activity.Execute"
	StateJobExecuteServiceTask = ".State.Job.Execute.ServiceTask"
	StateJobExecuteUserTask    = ".State.Job.Execute.UserTask"
	StateJobExecuteManualTask  = ".State.Job.Execute.ManualTask"
	StateJobExecuteSendMessage = ".State.Job.Execute.SendMessage"

	StateTraversalComplete      = ".State.Traversal.Complete"
	StateActivityComplete       = ".State.Activity.Complete"
	StateActivityAbort          = ".State.Activity.Abort"
	StateJobCompleteServiceTask = ".State.Job.Complete.ServiceTask"
	StateExecutionComplete      = ".State.Execution.Complete"
	StateProcessTerminated      = ".State.Process.Terminated"
	StateJobAbortServiceTask    = ".State.Job.Abort.ServiceTask"
	StateJobCompleteUserTask    = ".State.Job.Complete.UserTask"
	StateJobCompleteManualTask  = ".State.Job.Complete.ManualTask"
	StateJobCompleteSendMessage = ".State.Job.Complete.SendMessage"
	StateLog                    = ".State.Log."
)

// NatsConfig holds the current configuration of the SHAR Telemetry Server
//
//go:embed nats-config.yaml
var NatsConfig string

var startActions = []string{
	StateExecutionExecute, StateProcessExecute, StateTraversalExecute, StateActivityExecute,
	StateJobExecuteServiceTask, StateJobExecuteUserTask, StateJobExecuteManualTask, StateJobExecuteSendMessage,
}

var endActions = []string{
	StateTraversalComplete, StateActivityComplete, StateActivityAbort, StateJobCompleteServiceTask,
	StateExecutionComplete, StateProcessTerminated, StateJobAbortServiceTask, StateJobCompleteUserTask,
	StateJobCompleteManualTask, StateJobCompleteSendMessage,
}

var endStartActionMapping = map[string]string{
	StateTraversalComplete:      StateTraversalExecute,
	StateActivityComplete:       StateActivityExecute,
	StateActivityAbort:          StateActivityExecute,
	StateJobCompleteServiceTask: StateJobExecuteServiceTask,
	StateExecutionComplete:      StateExecutionExecute,
	StateProcessTerminated:      StateProcessExecute,
	StateJobAbortServiceTask:    StateJobExecuteServiceTask,
	StateJobCompleteUserTask:    StateJobExecuteUserTask,
	StateJobCompleteManualTask:  StateJobExecuteManualTask,
	StateJobCompleteSendMessage: StateJobExecuteSendMessage,
}

// Server is the shar server type responsible for hosting the telemetry server.
type Server struct {
	js     nats.JetStreamContext
	spanKV nats.KeyValue
	res    *resource.Resource
	exp    Exporter
	wfi    nats.KeyValue

	wfStateCounter       metric.Int64Counter
	wfStateUpDownCounter metric.Int64UpDownCounter
}

// New creates a new telemetry server.
func New(ctx context.Context, nc *nats.Conn, js nats.JetStreamContext, storageType nats.StorageType, exp Exporter) *Server {
	// Define our resource
	res := resource.NewWithAttributes(
		semconv.SchemaURL,
		semconv.ServiceNameKey.String(service),
		attribute.String("environment", environment),
		attribute.Int64("ID", id),
	)

	if err := setup.Nats(ctx, nc, js, storageType, NatsConfig, true); err != nil {
		//if err := common.EnsureBuckets(js, nats.FileStorage, []string{"WORKFLOW_TRACE"}); err != nil {
		//	panic(err)
		//}
		//
		//if err := ensureConsumer(js, workflowTelemetryStreamName, &nats.ConsumerConfig{
		//	Durable:       "Tracing",
		//	Description:   "Sequential Trace Consumer",
		//	DeliverPolicy: nats.DeliverAllPolicy,
		//	FilterSubject: "WORKFLOW-TELEMETRY.>",
		//	AckPolicy:     nats.AckExplicitPolicy,
		//	MaxAckPending: 1,
		//}); err != nil {
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
		slog.Error("err getting meter provider meter counter", "err", err.Error())
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

	kv, err := s.js.KeyValue(messages.KvTracking)
	if err != nil {
		return fmt.Errorf("listen failed to attach to tracking key value database: %w", err)
	}
	s.spanKV = kv
	kv, err = s.js.KeyValue(messages.KvInstance)
	if err != nil {
		return fmt.Errorf("listen failed to attach to instance key value database, is SHAR running on this cluster?: %w", err)
	}
	s.wfi = kv
	err = common.Process(ctx, s.js, "WORKFLOW_TELEMETRY", "telemetry", closer, "WORKFLOW.*.State.>", "Tracing", 1, s.workflowTrace)
	if err != nil {
		return fmt.Errorf("listen failed to start telemetry handler: %w", err)
	}
	return nil
}

var empty8 = [8]byte{}

func (s *Server) workflowTrace(ctx context.Context, log *slog.Logger, msg *nats.Msg) (bool, error) {
	state, done, err2 := s.decodeState(ctx, msg)
	if done {
		return done, err2
	}

	switch {
	case strings.HasSuffix(msg.Subject, StateExecutionExecute):
		s.incrementActionCounter(ctx, StateExecutionExecute)
		s.changeActionUpDownCounter(ctx, 1, StateExecutionExecute)

		if err := s.saveSpan(ctx, "Execution Start", state, state); err != nil {
			return true, nil
		}
	case strings.HasSuffix(msg.Subject, StateProcessExecute):
		s.incrementActionCounter(ctx, StateProcessExecute)
		s.changeActionUpDownCounter(ctx, 1, StateProcessExecute)

		if err := s.saveSpan(ctx, "Process Start", state, state); err != nil {
			return true, nil
		}
	case strings.HasSuffix(msg.Subject, StateTraversalExecute):
	case strings.HasSuffix(msg.Subject, StateActivityExecute):
		if err := s.spanStart(ctx, state, msg.Subject); err != nil {
			return true, nil
		}
	case strings.Contains(msg.Subject, StateJobExecuteServiceTask),
		strings.HasSuffix(msg.Subject, StateJobExecuteUserTask),
		strings.HasSuffix(msg.Subject, StateJobExecuteManualTask),
		strings.Contains(msg.Subject, StateJobExecuteSendMessage):
		if err := s.spanStart(ctx, state, msg.Subject); err != nil {
			return true, nil
		}
	case strings.HasSuffix(msg.Subject, StateTraversalComplete):
	case strings.HasSuffix(msg.Subject, StateActivityComplete),
		strings.HasSuffix(msg.Subject, StateActivityAbort):
		if err := s.spanEnd(ctx, "Activity: "+state.ElementId, state, msg.Subject); err != nil {
			var escape *AbandonOpError
			if errors.As(err, &escape) {
				log.Error("saving Activity.Complete operation abandoned", err,
					slog.String(keys.ExecutionID, state.ExecutionId),
					slog.String(keys.TrackingID, common.TrackingID(state.Id).ID()),
					slog.String(keys.ParentTrackingID, common.TrackingID(state.Id).ParentID()),
					slog.String(keys.ElementType, state.ElementType),
				)
				return true, err
			}
			return true, nil
		}
	case strings.Contains(msg.Subject, StateJobCompleteServiceTask),
		strings.Contains(msg.Subject, StateExecutionComplete),
		strings.Contains(msg.Subject, StateProcessTerminated),
		strings.Contains(msg.Subject, StateJobAbortServiceTask),
		strings.Contains(msg.Subject, StateJobCompleteUserTask),
		strings.Contains(msg.Subject, StateJobCompleteManualTask),
		strings.Contains(msg.Subject, StateJobCompleteSendMessage):

		if err := s.spanEnd(ctx, "Job: "+state.ElementType, state, msg.Subject); err != nil {
			var escape *AbandonOpError
			if errors.As(err, &escape) {
				log.Error("span end", err,
					slog.String(keys.ExecutionID, state.ExecutionId),
					slog.String(keys.TrackingID, common.TrackingID(state.Id).ID()),
					slog.String(keys.ParentTrackingID, common.TrackingID(state.Id).ParentID()),
					slog.String(keys.ElementType, state.ElementType),
				)
				return true, err
			}
			return true, nil
		}
	case strings.Contains(msg.Subject, StateJobCompleteSendMessage):
	case strings.Contains(msg.Subject, StateLog):

	//case strings.HasSuffix(msg.Subject, ".State.Execution.Complete"):
	//case strings.HasSuffix(msg.Subject, ".State.Execution.Terminated"):
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

func (s *Server) decodeState(ctx context.Context, msg *nats.Msg) (*model.WorkflowState, bool, error) {
	log := logx.FromContext(ctx)
	state := &model.WorkflowState{}
	err := proto.Unmarshal(msg.Data, state)
	if err != nil {
		log.Error("unmarshal span", err)
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
		log.Error("load span state:", err, slog.String(keys.TrackingID, common.TrackingID(state.Id).ID()))
		return abandon(err)
	}
	state.ExecutionId = oldState.ExecutionId
	state.Id = oldState.Id
	state.WorkflowId = oldState.WorkflowId
	state.ElementId = oldState.ElementId
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
		keys.ElementType: &oldState.ElementType,
		keys.WorkflowID:  &oldState.WorkflowId,
		keys.ExecutionID: &oldState.ExecutionId,
		keys.Condition:   oldState.Condition,
		keys.Execute:     oldState.Execute,
		keys.State:       &st,
		"trackingId":     &id,
		"parentTrId":     &pid,
	}

	kv, err := vars.Decode(ctx, newState.Vars)
	if err != nil {
		return abandon(err)
	}

	for k, v := range kv {
		val := fmt.Sprintf("%+v", v)
		at["var."+k] = &val
	}

	attrs := buildAttrs(at)
	err = s.exp.ExportSpans(
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
				InstrumentationLib: instrumentation.Library{
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
	err = s.spanKV.Delete(common.TrackingID(oldState.Id).ID())
	if err != nil {
		id := common.TrackingID(oldState.Id).ID()
		log.Warn("delete the cached span", err, slog.String(keys.TrackingID, id))
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

func SetupMetrics(ctx context.Context, serviceName string) (*sdkmetric.MeterProvider, error) {
	//c, err := getTls()
	//if err != nil {
	//	return nil, err
	//}

	exporter, err := otlpmetrichttp.New(
		ctx,
		otlpmetrichttp.WithInsecure(), //just for local testing, probably should be TLS in deployed envs
		//otlpmetricgrpc.WithEndpoint("localhost:4317"),
		//otlpmetricgrpc.WithTLSCredentials(
		//	// mutual tls.
		//	credentials.NewTLS(c),
		//),
	)
	if err != nil {
		return nil, err
	}

	// labels/tags/resources that are common to all metrics.
	resource := resource.NewWithAttributes(
		semconv2.SchemaURL,
		semconv2.ServiceNameKey.String(serviceName),
		attribute.String("app", "shar-telemetry"),
	)

	mp := sdkmetric.NewMeterProvider(
		sdkmetric.WithResource(resource),
		sdkmetric.WithReader(
			// collects and exports metric data every N seconds.
			sdkmetric.NewPeriodicReader(exporter, sdkmetric.WithInterval(10*time.Second)),
		),
	)

	otel.SetMeterProvider(mp)

	return mp, nil
}
