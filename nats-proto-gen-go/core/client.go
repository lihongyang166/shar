package core

import (
	"context"
	"errors"
	"fmt"
	"github.com/nats-io/nats.go"
	"gitlab.com/shar-workflow/shar/client/api"
	"google.golang.org/protobuf/proto"
	"log/slog"
	"strconv"
	"strings"
	"time"
)

// Call provides the functionality to call NATS req/rep APIs
func Call[T proto.Message, U proto.Message](ctx context.Context, con *nats.Conn, middleware []Handler, errorHandler ErrorHandler, subject string, command T, ret U) error {
	b, err := proto.Marshal(command)
	if err != nil {
		return fmt.Errorf("marshal proto for call API: %w", err)
	}
	msg := nats.NewMsg(subject)
	for _, handler := range middleware {
		var handlerError error
		if ctx, handlerError = handler(ctx, msg); handlerError != nil {
			slog.Error("middleware", msg.Subject+":", handlerError)
			return err
		}
	}
	msg.Data = b
	res, err := con.RequestMsg(msg, time.Second*60)
	if err != nil {
		if errors.Is(err, nats.ErrNoResponders) {
			err = fmt.Errorf("client: server is offline or missing from the current nats server")
		}
		return fmt.Errorf("API call: %w", err)
	}
	if len(res.Data) > 4 && string(res.Data[0:4]) == ErrorPrefix {
		em := strings.Split(string(res.Data), ErrorSeparator)
		e := strings.Split(em[0], "\x01")
		i, err := strconv.Atoi(e[1])
		if err != nil {
			i = 0
		}
		ae := &api.Error{Code: i, Message: em[1]}
		if errorHandler != nil {
			return errorHandler(err, msg)
		}
		return ae
	}
	if err := proto.Unmarshal(res.Data, ret); err != nil {
		return fmt.Errorf("unmarshal proto for call API: %w", err)
	}
	return nil
}
