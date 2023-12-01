package common

import (
	"context"
	"fmt"
	"github.com/nats-io/nats.go"
	"gitlab.com/shar-workflow/shar/common/logx"
	"gitlab.com/shar-workflow/shar/model"
	"gitlab.com/shar-workflow/shar/server/errors/keys"
	"gitlab.com/shar-workflow/shar/server/messages"
	"log/slog"
	"os"
)

var hostName string

type LogPublisher interface {
	Publish(ctx context.Context, lr *model.LogRequest) error
}

type NatsLogPublisher struct {
	Conn *nats.Conn
}

func (nlp *NatsLogPublisher) Publish(ctx context.Context, lr *model.LogRequest) error {
	if err := PublishObj(ctx, nlp.Conn, messages.WorkflowTelemetryLog, lr, nil); err != nil {
		return fmt.Errorf("publish object: %w", err)
	}
	return nil
}

type HandlerOptions struct {
	Level slog.Leveler
}

type SharHandler struct {
	opts         HandlerOptions
	logPublisher LogPublisher
	groupPrefix  string
	attrs        []slog.Attr
}

func (sh *SharHandler) Enabled(_ context.Context, level slog.Level) bool {
	minLevel := slog.LevelInfo
	if sh.opts.Level != nil {
		minLevel = sh.opts.Level.Level()
	}
	return level >= minLevel
}

func (sh *SharHandler) Handle(ctx context.Context, r slog.Record) error {
	attr := map[string]string{}
	r.Attrs(func(a slog.Attr) bool {
		attr[a.Key] = a.Value.String()
		return true
	})

	for _, a := range sh.attrs {
		attr[a.Key] = a.Value.String()
	}

	lr := &model.LogRequest{
		Hostname:   hostName,
		ClientId:   "",
		TrackingId: nil,
		Level:      int32(r.Level),
		Time:       r.Time.UnixMilli(),
		Source:     model.LogSource_logSourceEngine,
		Message:    r.Message,
		Attributes: attr,
	}

	err := sh.logPublisher.Publish(ctx, lr)
	if err != nil {
		return err
	}

	return nil
}

func withGroupPrefix(groupPrefix string, attr slog.Attr) slog.Attr {
	if groupPrefix != "" {
		attr.Key = groupPrefix + attr.Key
	}
	return attr
}

func (sh *SharHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	for i, attr := range attrs {
		attrs[i] = withGroupPrefix(sh.groupPrefix, attr)
	}

	return &SharHandler{
		opts:         sh.opts,
		groupPrefix:  sh.groupPrefix,
		attrs:        append(sh.attrs, attrs...),
		logPublisher: sh.logPublisher,
	}
}

func (sh *SharHandler) WithGroup(name string) slog.Handler {
	if name == "" {
		return sh
	}
	prefix := name + "."
	if sh.groupPrefix != "" {
		prefix = sh.groupPrefix + prefix
	}

	return &SharHandler{
		opts:         sh.opts,
		attrs:        sh.attrs,
		groupPrefix:  prefix,
		logPublisher: sh.logPublisher,
	}

}

func NewSharHandler(opts HandlerOptions, logPublisher LogPublisher) slog.Handler {
	var err error
	hostName, err = os.Hostname()
	if err != nil {
		panic(err)
	}

	sharHandler := &SharHandler{opts: opts, logPublisher: logPublisher}
	return sharHandler
}

func ContextLoggerWithWfState(ctx context.Context, state *model.WorkflowState) (context.Context, *slog.Logger) {
	logger := logx.FromContext(ctx).
		With(wfStatePrefix(keys.ExecutionID), state.ExecutionId).
		With(wfStatePrefix(keys.TrackingID), TrackingID(state.Id).ID()).
		With(wfStatePrefix(keys.ParentTrackingID), TrackingID(state.Id).ParentID())
	return logx.NewContext(ctx, logger), logger
}

func wfStatePrefix(keyName string) string {
	return fmt.Sprintf("%s%s", logx.WfStateLoggingKeyPrefix, keyName)
}
