package common

import (
	"context"
	"errors"
	"fmt"
	"github.com/nats-io/nats.go"
	"github.com/stretchr/testify/assert"
	"gitlab.com/shar-workflow/shar/common"
	integration_support "gitlab.com/shar-workflow/shar/internal/integration-support"
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
