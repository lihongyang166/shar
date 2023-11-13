package core

import (
	"context"
	"fmt"
	"github.com/nats-io/nats.go"
	"google.golang.org/grpc/codes"
	"google.golang.org/protobuf/proto"
	"log/slog"
	"sync"
)

const (
	ErrorPrefix    = "ERR\x01" // ErrorPrefix ERR(Start of Heading) Denotes an API error.
	ErrorSeparator = "\x02"    // ErrorSeparator (Start of Text) Denotes the start of the API error message.
)

type Handler func(ctx context.Context, msg *nats.Msg) (context.Context, error)

var DefaultHandler = make([]Handler, 0)

type ErrorHandler func(err error, msg *nats.Msg) error

var DefaultErrorHandler ErrorHandler = func(err error, msg *nats.Msg) error {
	c := codes.Unknown
	ErrorResponse(msg, c, err.Error())
	return fmt.Errorf("API call: %w", err)
}

func Listen[T proto.Message, U proto.Message](con *nats.Conn, panicRecovery bool, subList *sync.Map, middleware []Handler, errorHandler ErrorHandler, subject string, req T, fn func(ctx context.Context, req T) (U, error)) error {
	sub, err := con.QueueSubscribe(subject, subject, func(msg *nats.Msg) {
		ctx := context.Background()
		for _, handler := range middleware {
			var handlerError error
			if ctx, handlerError = handler(ctx, msg); handlerError != nil {
				slog.Error("middleware", msg.Subject+":", handlerError)
				return
			}
		}
		if err := CallServerAPI(ctx, panicRecovery, errorHandler, req, msg, fn); err != nil {
			slog.Error("API call "+subject+":", err)
		}
	})
	if err != nil {
		return fmt.Errorf("subscribe to %s: %w", subject, err)
	}
	subList.Store(sub, struct{}{})
	return nil
}

func CallServerAPI[T proto.Message, U proto.Message](ctx context.Context, panicRecovery bool, errorHandler ErrorHandler, container T, msg *nats.Msg, fn func(ctx context.Context, req T) (U, error)) error {
	if panicRecovery {
		defer recoverAPIpanic(msg)
	}
	if err := proto.Unmarshal(msg.Data, container); err != nil {
		ErrorResponse(msg, codes.InvalidArgument, err.Error())
		return fmt.Errorf("unmarshal message data during callAPI: %w", err)
	}
	resMsg, err := fn(ctx, container)
	if err != nil {
		return errorHandler(err, msg)
	}
	res, err := proto.Marshal(resMsg)
	if err != nil {
		ErrorResponse(msg, codes.InvalidArgument, err.Error())
		return fmt.Errorf("unmarshal API response: %w", err)
	}
	if err := msg.Respond(res); err != nil {
		ErrorResponse(msg, codes.FailedPrecondition, err.Error())
		return fmt.Errorf("API response: %w", err)
	}
	return nil
}

func RecoverAPIpanic(msg *nats.Msg) {
	if r := recover(); r != nil {
		ErrorResponse(msg, codes.Internal, r)
		slog.Info("recovered from", "error", r)
	}
}

func ErrorResponse(m *nats.Msg, code codes.Code, msg any) {
	if err := m.Respond(ApiError(code, msg)); err != nil {
		slog.Error("send error response: "+string(ApiError(codes.Internal, msg)), err)
	}
}

func ApiError(code codes.Code, msg any) []byte {
	err := fmt.Sprintf("%s%d%s%+v", ErrorPrefix, code, ErrorSeparator, msg)
	return []byte(err)
}

func recoverAPIpanic(msg *nats.Msg) {
	if r := recover(); r != nil {
		ErrorResponse(msg, codes.Internal, r)
		slog.Info("recovered from ", "error", r)
	}
}
