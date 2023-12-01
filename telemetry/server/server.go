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
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/sdk/instrumentation"
	"go.opentelemetry.io/otel/sdk/resource"
	tracesdk "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.10.0"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/protobuf/proto"
	"log/slog"
	"strings"
	"time"
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

// Server is the shar server type responsible for hosting the telemetry server.
type Server struct {
	js     nats.JetStreamContext
	spanKV nats.KeyValue
	res    *resource.Resource
	exp    Exporter
	wfi    nats.KeyValue
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
		panic(err)
	}

	return &Server{
		js:  js,
		res: res,
		exp: exp,
	}
}

// Listen starts the telemtry server.
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
	err = common.Process(ctx, s.js, "WORKFLOW-TELEMETRY", "telemetry", closer, "WORKFLOW-TELEMETRY.>", "Tracing", 1, s.workflowTrace)
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
	case strings.HasSuffix(msg.Subject, ".State.Execution.Execute"):
		if err := s.saveSpan(ctx, "Execution Start", state, state); err != nil {
			return true, nil
		}
	case strings.HasSuffix(msg.Subject, ".State.Process.Execute"):
		if err := s.saveSpan(ctx, "Process Start", state, state); err != nil {
			return true, nil
		}
	case strings.HasSuffix(msg.Subject, ".State.Traversal.Execute"):
	case strings.HasSuffix(msg.Subject, ".State.Activity.Execute"):
		if err := s.spanStart(ctx, state); err != nil {
			return true, nil
		}
	case strings.Contains(msg.Subject, ".State.Job.Execute.ServiceTask"),
		strings.HasSuffix(msg.Subject, ".State.Job.Execute.UserTask"),
		strings.HasSuffix(msg.Subject, ".State.Job.Execute.ManualTask"),
		strings.Contains(msg.Subject, ".State.Job.Execute.SendMessage"):
		if err := s.spanStart(ctx, state); err != nil {
			return true, nil
		}
	case strings.HasSuffix(msg.Subject, ".State.Traversal.Complete"):
	case strings.HasSuffix(msg.Subject, ".State.Activity.Complete"),
		strings.HasSuffix(msg.Subject, ".State.Activity.Abort"):
		if err := s.spanEnd(ctx, "Activity: "+state.ElementId, state); err != nil {
			var escape *AbandonOpError
			if errors.As(err, &escape) {
				log.Error("saving Activity.Complete operation abandoned", err,
					slog.String(keys.ExecutionID, state.ExecutionId),
					slog.String(keys.TrackingID, common.TrackingID(state.Id).ID()),
					slog.String(keys.ParentTrackingID, common.TrackingID(state.Id).ParentID()),
				)
				return true, err
			}
			return true, nil
		}
	case strings.Contains(msg.Subject, ".State.Job.Complete.ServiceTask"),
		strings.Contains(msg.Subject, ".State.Execution.Complete"),
		strings.Contains(msg.Subject, ".State.Process.Terminated"),
		strings.Contains(msg.Subject, ".State.Job.Abort.ServiceTask"),
		strings.Contains(msg.Subject, ".State.Job.Complete.UserTask"),
		strings.Contains(msg.Subject, ".State.Job.Complete.ManualTask"),
		strings.Contains(msg.Subject, ".State.Job.Complete.SendMessage"):
		if err := s.spanEnd(ctx, "Job: "+state.ElementType, state); err != nil {
			var escape *AbandonOpError
			if errors.As(err, &escape) {
				log.Error("saving Job.Complete operation abandoned", err,
					slog.String(keys.ExecutionID, state.ExecutionId),
					slog.String(keys.TrackingID, common.TrackingID(state.Id).ID()),
					slog.String(keys.ParentTrackingID, common.TrackingID(state.Id).ParentID()),
				)
				return true, err
			}
			return true, nil
		}
	case strings.Contains(msg.Subject, ".State.Job.Complete.SendMessage"):
	case strings.Contains(msg.Subject, ".State.Log."):

	//case strings.HasSuffix(msg.Subject, ".State.Execution.Complete"):
	//case strings.HasSuffix(msg.Subject, ".State.Execution.Terminated"):
	default:

	}
	return true, nil
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

func (s *Server) spanStart(ctx context.Context, state *model.WorkflowState) error {
	err := common.SaveObj(ctx, s.spanKV, common.TrackingID(state.Id).ID(), state)
	if err != nil {
		return fmt.Errorf("span-start failed fo save object: %w", err)
	}
	return nil
}

func (s *Server) spanEnd(ctx context.Context, name string, state *model.WorkflowState) error {
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
