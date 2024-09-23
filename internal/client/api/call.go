package api

import (
	"context"
	"errors"
	"fmt"
	version2 "github.com/hashicorp/go-version"
	"github.com/nats-io/nats.go"
	"github.com/segmentio/ksuid"
	"gitlab.com/shar-workflow/shar/client/api"
	"gitlab.com/shar-workflow/shar/common"
	"gitlab.com/shar-workflow/shar/common/ctxkey"
	"gitlab.com/shar-workflow/shar/common/header"
	"gitlab.com/shar-workflow/shar/common/logx"
	"gitlab.com/shar-workflow/shar/common/middleware"
	"gitlab.com/shar-workflow/shar/common/version"
	"gitlab.com/shar-workflow/shar/internal"
	errors2 "gitlab.com/shar-workflow/shar/server/errors"
	"google.golang.org/grpc/codes"
	"google.golang.org/protobuf/proto"
	"reflect"
	"strconv"
	"strings"
	"time"
)

// Call provides the functionality to call shar APIs
func Call[T proto.Message, U proto.Message](ctx context.Context, con *nats.Conn, subject string, expectCompat *version2.Version, sendMiddleware []middleware.Send, command T, ret U) error {

	if ctx.Value(ctxkey.SharNamespace) == nil {
		panic("contextless call")
	}
	b, err := proto.Marshal(command)
	if err != nil {
		return fmt.Errorf("marshal proto for call API: %w", err)
	}
	msg := nats.NewMsg(subject)
	ctx = context.WithValue(ctx, logx.CorrelationContextKey, ksuid.New().String())
	if err := header.FromCtxToMsgHeader(ctx, &msg.Header); err != nil {
		return fmt.Errorf("attach headers to outgoing API message: %w", err)
	}
	if ns := ctx.Value(ctxkey.SharNamespace); ns != nil {
		msg.Header.Add(header.SharNamespace, ns.(string))
	}
	msg.Header.Add(header.NatsVersionHeader, version.Version)
	if expectCompat != nil {
		msg.Header.Add(header.NatsCompatHeader, expectCompat.String())
	} else {
		msg.Header.Add(header.NatsCompatHeader, "v0.0.0")
	}
	for _, i := range sendMiddleware {
		mwrap := common.NewNatsMsgWrapper(msg)
		if err := i(ctx, mwrap); err != nil {
			return fmt.Errorf("send middleware %s: %w", reflect.TypeOf(i).Name(), err)
		}
	}
	msg.Data = b
	res, err := con.RequestMsg(msg, time.Second*60)
	if err != nil {
		if errors.Is(err, nats.ErrNoResponders) {
			err = fmt.Errorf("shar-client: shar server is offline or missing from the current nats server")
		}
		return fmt.Errorf("API call: %w", err)
	}
	if len(res.Data) > 4 && string(res.Data[0:4]) == internal.ErrorPrefix {

		em := strings.Split(string(res.Data), internal.ErrorSeparator)
		e := strings.Split(em[0], "\x01")
		i, err := strconv.Atoi(e[1])
		if err != nil {
			i = 0
		}
		ae := &api.Error{Code: i, Message: em[1]}
		if codes.Code(i) == codes.Internal { //nolint:gosec
			return &errors2.ErrWorkflowFatal{Err: ae}
		}
		return ae
	}
	if err := proto.Unmarshal(res.Data, ret); err != nil {
		return fmt.Errorf("unmarshal proto for call API: %w", err)
	}
	return nil
}

// CallReturnStream provides the functionality to call shar APIs and receive a streaming response
func CallReturnStream[T proto.Message, U proto.Message](ctx context.Context, con *nats.Conn, subject string, expectCompat *version2.Version, sendMiddleware []middleware.Send, command T, container U, fn func(ret U) error) error {

	if ctx.Value(ctxkey.SharNamespace) == nil {
		panic("contextless call")
	}
	b, err := proto.Marshal(command)
	if err != nil {
		return fmt.Errorf("marshal proto for call API: %w", err)
	}
	msg := nats.NewMsg(subject)
	ctx = context.WithValue(ctx, logx.CorrelationContextKey, ksuid.New().String())
	if err := header.FromCtxToMsgHeader(ctx, &msg.Header); err != nil {
		return fmt.Errorf("attach headers to outgoing API message: %w", err)
	}
	if ns := ctx.Value(ctxkey.SharNamespace); ns != nil {
		msg.Header.Add(header.SharNamespace, ns.(string))
	}
	msg.Header.Add(header.NatsVersionHeader, version.Version)
	if expectCompat != nil {
		msg.Header.Add(header.NatsCompatHeader, expectCompat.String())
	} else {
		msg.Header.Add(header.NatsCompatHeader, "v0.0.0")
	}
	for _, i := range sendMiddleware {
		mWrap := common.NewNatsMsgWrapper(msg)
		if err := i(ctx, mWrap); err != nil {
			return fmt.Errorf("send middleware %s: %w", reflect.TypeOf(i).Name(), err)
		}
	}
	msg.Data = b
	err = common.StreamingReplyClient(ctx, con, msg, func(res *nats.Msg) error {
		if len(res.Data) > 4 && string(res.Data[0:4]) == internal.ErrorPrefix {
			em := strings.Split(string(res.Data), internal.ErrorSeparator)
			e := strings.Split(em[0], "\x01")
			i, err := strconv.Atoi(e[1])
			if err != nil {
				i = 0
			}
			// nolint
			ae := &api.Error{Code: i, Message: em[1]}
			if codes.Code(i) == codes.Internal { //nolint:gosec
				return &errors2.ErrWorkflowFatal{Err: ae}
			}
			return ae
		}
		// Create new proto instance of the return variable
		ct := container.ProtoReflect().New().Interface().(U)

		if err := proto.Unmarshal(res.Data, ct); err != nil {
			return fmt.Errorf("unmarshal proto for call API: %w", err)
		}
		err := fn(ct)
		// TODO: trigger cancellation on error
		if err != nil {
			// TODO: translate error
			return fmt.Errorf("call API: %w", err)
		}
		return nil
	})
	if err != nil {
		if errors.Is(err, nats.ErrNoResponders) {
			err = fmt.Errorf("shar-client: shar server is offline or missing from the current nats server")
		}
		if i := strings.Index(err.Error(), internal.ErrorPrefix); i != -1 {
			svrErr := err.Error()[i:]
			p1 := strings.Split(svrErr, string(internal.ErrorPrefix[len(internal.ErrorPrefix)-1]))
			p2 := strings.Split(p1[1], internal.ErrorSeparator)
			code, convErr := strconv.Atoi(p2[0])
			if convErr != nil {
				return fmt.Errorf("bad server errot: %w", err)
			}
			err = &api.Error{Code: code, Message: p2[1]}
		}
		return fmt.Errorf("API call: %w", err)
	}
	return nil
}
