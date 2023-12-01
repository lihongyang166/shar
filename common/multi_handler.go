package common

import (
	"context"
	"log/slog"
)

type MultiHandler struct {
	Handlers []slog.Handler
	opts     HandlerOptions
}

func (mh *MultiHandler) Enabled(_ context.Context, level slog.Level) bool {
	return true
	// always enabled as this is a composite handler. We check for each handler in Handle() anyway
}

func (mh *MultiHandler) Handle(ctx context.Context, r slog.Record) error {
	for _, h := range mh.Handlers {
		if h.Enabled(ctx, r.Level) {
			h.Handle(ctx, r)
		}
	}

	return nil
}

func (mh *MultiHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	handlersWithAttrs := []slog.Handler{}
	for _, h := range mh.Handlers {
		handlersWithAttrs = append(handlersWithAttrs, h.WithAttrs(attrs))
	}

	return &MultiHandler{
		Handlers: handlersWithAttrs,
		opts:     mh.opts,
	}
}

func (mh *MultiHandler) WithGroup(name string) slog.Handler {
	hWithGroup := make([]slog.Handler, 0, len(mh.Handlers))
	for _, h := range mh.Handlers {
		hWithGroup = append(hWithGroup, h.WithGroup(name))
	}

	return &MultiHandler{
		Handlers: hWithGroup,
		opts:     mh.opts,
	}
}

func NewMultiHandler(handlers []slog.Handler, handlerOptions HandlerOptions) *MultiHandler {
	return &MultiHandler{
		Handlers: handlers,
		opts:     handlerOptions,
	}
}
