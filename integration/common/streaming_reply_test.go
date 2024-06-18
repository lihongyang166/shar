package common

import (
	"context"
	"errors"
	"fmt"
	"github.com/nats-io/nats.go"
	"github.com/stretchr/testify/assert"
	task "gitlab.com/shar-workflow/shar/client"
	"gitlab.com/shar-workflow/shar/common"
	"gitlab.com/shar-workflow/shar/common/ctxkey"
	"gitlab.com/shar-workflow/shar/common/middleware"
	"gitlab.com/shar-workflow/shar/common/telemetry"
	api2 "gitlab.com/shar-workflow/shar/internal/client/api"
	integration_support "gitlab.com/shar-workflow/shar/internal/integration-support"
	"gitlab.com/shar-workflow/shar/model"
	"gitlab.com/shar-workflow/shar/server/api"
	"strconv"
	"sync"
	"testing"
	"time"
)

func TestStreamingReply(t *testing.T) {

	it := integration_support.NewIntegration(false, "common", nil)
	it.Setup()
	defer it.Teardown()
	nc, err := it.GetNats()
	assert.NoError(t, err)
	var topic = "Test.Topic"

	sent := 0
	received := 0

	// emulate server
	lSub, err := common.StreamingReplyServer(nc, topic, func(msg *nats.Msg, ret chan *nats.Msg, errs chan error) {
		for i := byte(0); i < 5; i++ {
			sent++
			msg := nats.NewMsg("reply")
			msg.Data = []byte{i}
			ret <- msg
		}
	})
	assert.NoError(t, err)
	defer func() {
		err := lSub.Unsubscribe()
		assert.NoError(t, err)
	}()

	//i := 0
	// Emulate Client
	msg := nats.NewMsg(topic)
	ctx := context.Background()
	err = common.StreamingReplyClient(ctx, nc, msg, func(msg *nats.Msg) error {
		received++
		return nil
	})
	assert.NoError(t, err)
	assert.Equal(t, sent, received)
}

func TestStreamingReplyClientError(t *testing.T) {

	it := integration_support.NewIntegration(false, "common", nil)
	it.Setup()
	defer it.Teardown()
	nc, err := it.GetNats()
	assert.NoError(t, err)
	var topic = "Test.Topic"

	sent := 0
	received := 0

	// emulate server
	lSub, err := common.StreamingReplyServer(nc, topic, func(msg *nats.Msg, ret chan *nats.Msg, errs chan error) {
		for i := byte(0); i < 100; i++ {
			sent++
			msg := nats.NewMsg("reply")
			msg.Data = []byte{i}
			ret <- msg
			time.Sleep(10 * time.Millisecond)
		}
	})
	assert.NoError(t, err)
	defer func() {
		err := lSub.Unsubscribe()
		assert.NoError(t, err)
	}()

	//i := 0
	// Emulate Client
	msg := nats.NewMsg(topic)
	ctx := context.Background()
	err = common.StreamingReplyClient(ctx, nc, msg, func(msg *nats.Msg) error {
		received++
		if received == 10 {
			return fmt.Errorf("test receive error: %w", errors.New("cancelled"))
		}
		return nil
	})
	assert.Error(t, err)
	assert.Equal(t, received, 10)
	assert.Less(t, sent, 100)
}

func TestStreamingReplyClientCancel(t *testing.T) {
	it := integration_support.NewIntegration(false, "common", nil)
	it.Setup()
	defer it.Teardown()
	nc, err := it.GetNats()
	assert.NoError(t, err)
	var topic = "Test.Topic"

	sent := 0
	received := 0

	// emulate server
	lSub, err := common.StreamingReplyServer(nc, topic, func(msg *nats.Msg, ret chan *nats.Msg, errs chan error) {
		for i := byte(0); i < 100; i++ {
			sent++
			msg := nats.NewMsg("reply")
			msg.Data = []byte{i}
			ret <- msg
			time.Sleep(10 * time.Millisecond)
		}
	})
	assert.NoError(t, err)
	defer func() {
		err := lSub.Unsubscribe()
		assert.NoError(t, err)
	}()

	//i := 0
	// Emulate Client
	msg := nats.NewMsg(topic)
	ctx := context.Background()
	err = common.StreamingReplyClient(ctx, nc, msg, func(msg *nats.Msg) error {
		received++
		if received == 10 {
			return common.ErrStreamCancel
		}
		return nil
	})
	assert.NoError(t, err)
	assert.Equal(t, received, 10)
	assert.Less(t, sent, 100)
}

func TestStreamingReplyServerError(t *testing.T) {

	it := integration_support.NewIntegration(false, "common", nil)
	it.Setup()
	defer it.Teardown()
	nc, err := it.GetNats()
	assert.NoError(t, err)
	var topic = "Test.Topic"

	sent := 0
	received := 0

	// emulate server
	lSub, err := common.StreamingReplyServer(nc, topic, func(msg *nats.Msg, ret chan *nats.Msg, errs chan error) {
		for i := byte(0); i < 100; i++ {
			msg := nats.NewMsg("reply")
			msg.Data = []byte{i}

			if sent < 10 {
				sent++
				ret <- msg
			} else {
				errs <- errors.New("server error")
			}

		}
	})
	assert.NoError(t, err)
	defer func() {
		err := lSub.Unsubscribe()
		assert.NoError(t, err)
	}()

	//i := 0
	// Emulate Client
	msg := nats.NewMsg(topic)
	ctx := context.Background()
	err = common.StreamingReplyClient(ctx, nc, msg, func(msg *nats.Msg) error {
		received++
		return nil
	})
	assert.Error(t, err)
	assert.Equal(t, sent, received)
	assert.Equal(t, received, 10)
}

func TestStreamingAPI(t *testing.T) {
	it := integration_support.NewIntegration(false, "common", nil)
	it.Setup()
	defer it.Teardown()
	nc, err := it.GetNats()
	assert.NoError(t, err)
	client := task.New()
	ctx := context.Background()
	ctx = context.WithValue(ctx, ctxkey.SharNamespace, "default")
	err = client.Dial(ctx, it.NatsURL)
	assert.NoError(t, err)
	var topic = "Test.Topic"
	var topic2 = "Test.Topic.Err"
	subList := sync.Map{}
	serverMW := []middleware.Receive{
		telemetry.CtxWithTraceParentFromNatsMsgMiddleware(),
		telemetry.NatsMsgToCtxWithSpanMiddleware(),
	}
	err = api.ListenReturnStream(nc, true, &subList, topic, serverMW, &model.ListExecutionRequest{}, testAPIFunc)
	assert.NoError(t, err)
	err = api.ListenReturnStream(nc, true, &subList, topic2, serverMW, &model.ListExecutionRequest{}, testAPIFuncErr)
	assert.NoError(t, err)
	req := &model.ListExecutionRequest{}
	v := client.ExpectedCompatibleServerVersion
	assert.NoError(t, err)
	err = api2.CallReturnStream(ctx, nc, topic, v, []middleware.Send{telemetry.CtxSpanToNatsMsgMiddleware()}, req, &model.Execution{}, func(ret *model.Execution) error {
		fmt.Printf("%v\n", ret)
		return nil
	})
	assert.NoError(t, err)
	err = api2.CallReturnStream(ctx, nc, topic2, v, []middleware.Send{telemetry.CtxSpanToNatsMsgMiddleware()}, req, &model.Execution{}, func(ret *model.Execution) error {
		fmt.Printf("%v\n", ret)
		return nil
	})
	assert.ErrorContains(t, err, "code 13: test error")
	err = api2.CallReturnStream(ctx, nc, topic, v, []middleware.Send{telemetry.CtxSpanToNatsMsgMiddleware()}, req, &model.Execution{}, func(ret *model.Execution) error {
		return fmt.Errorf("call and return stream: %w", errors.New("one bang and the client is gone"))
	})
	assert.Error(t, err)
}

func testAPIFunc(ctx context.Context, req *model.ListExecutionRequest, res chan<- *model.Execution, errs chan<- error) {
	for i := 0; i < 5; i++ {
		ret := &model.Execution{}
		ret.ExecutionId = strconv.Itoa(i)
		ret.WorkflowId = strconv.Itoa(i)
		ret.ProcessInstanceId = []string{strconv.Itoa(i)}
		res <- ret
	}
}

func testAPIFuncErr(ctx context.Context, req *model.ListExecutionRequest, res chan<- *model.Execution, errs chan<- error) {
	for i := 0; i < 5; i++ {
		ret := &model.Execution{}
		ret.ExecutionId = strconv.Itoa(i)
		ret.WorkflowId = strconv.Itoa(i)
		ret.ProcessInstanceId = []string{strconv.Itoa(i)}
		if i == 4 {
			errs <- errors.New("test error")
			continue
		}
		res <- ret
	}
}
