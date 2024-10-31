package tests

import (
	"context"
	"fmt"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gitlab.com/shar-workflow/shar/common"
	"gitlab.com/shar-workflow/shar/internal/usertaskindexer"
	"gitlab.com/shar-workflow/shar/model"
	"google.golang.org/protobuf/proto"
	"testing"
	"time"
)

func Test_UserTaskIndex(t *testing.T) {
	r := make(chan struct{})
	nc, err := nats.Connect(tst.NatsURL)
	require.NoError(t, err)
	js, err := jetstream.New(nc)
	require.NoError(t, err)
	ctx := context.Background()
	ready := func() {
		close(r)
	}
	svr, err := usertaskindexer.New(nc, usertaskindexer.IndexOptions{Ready: ready, WarmupDelay: time.Second * 1})
	require.NoError(t, err)
	kv1, err := js.CreateOrUpdateKeyValue(ctx, jetstream.KeyValueConfig{Bucket: "default_WORKFLOW_USERTASK", Storage: jetstream.MemoryStorage, History: 2})
	require.NoError(t, err)
	kv2, err := js.CreateOrUpdateKeyValue(ctx, jetstream.KeyValueConfig{Bucket: "stacey_WORKFLOW_USERTASK", Storage: jetstream.MemoryStorage, History: 2})
	require.NoError(t, err)
	ut1 := &model.WorkflowState{
		Owners: []string{"gareth", "paul"},
		Groups: []string{"engineers"},
	}
	ut2 := &model.WorkflowState{
		Owners: []string{"julian", "peter"},
		Groups: []string{"engineers"},
	}
	ut3 := &model.WorkflowState{
		Owners: []string{"julian", "gareth"},
		Groups: []string{"engineers"},
	}
	ut4 := &model.WorkflowState{
		Owners: []string{"julian", "paul"},
		Groups: []string{"admins"},
	}
	bb, err := proto.Marshal(&model.WorkflowState{})
	require.NoError(t, err)
	err = nc.Publish("WORKFLOW.stacey.State.Job.Execute.UserTask", bb)
	require.NoError(t, err)
	err = common.SaveObj(ctx, kv1, "KEY1", ut1)
	require.NoError(t, err)
	err = common.SaveObj(ctx, kv1, "KEY2", ut2)
	require.NoError(t, err)
	err = common.SaveObj(ctx, kv2, "KEY3", ut3)
	require.NoError(t, err)
	err = common.SaveObj(ctx, kv2, "KEY4", ut4)
	require.NoError(t, err)
	err = svr.Start(ctx)
	require.NoError(t, err)
	fmt.Println(kv1, kv2)
	require.NoError(t, err)
	select {
	case <-r:
		fmt.Println("ready")
	case <-time.After(time.Second * 20):
		fmt.Println("timeout")
	}
	<-time.After(1 * time.Second)
	ret, retErr := svr.QueryByUser(ctx, "default", "gareth")
	testRes1 := make([]*model.WorkflowState, 0)
	for done := false; !done; {
		select {
		case err := <-retErr:
			if err == nil {
				done = true
				break
			}
			panic(err)
		case userTask := <-ret:
			testRes1 = append(testRes1, userTask)
		}
	}
	assert.Len(t, testRes1, 1)
	testRes2 := make([]*model.WorkflowState, 0)
	ret2, retErr := svr.QueryByUser(ctx, "stacey", "julian")
	for done := false; !done; {
		select {
		case err := <-retErr:
			if err == nil {
				done = true
				break
			}
			panic(err)
		case userTask := <-ret2:
			testRes2 = append(testRes2, userTask)
		}
	}
	assert.Len(t, testRes2, 2)
	err = common.Delete(ctx, kv2, "KEY4")
	require.NoError(t, err)
	testRes3 := make([]*model.WorkflowState, 0)
	ret3, retErr := svr.QueryByUser(ctx, "stacey", "julian")
	for done := false; !done; {
		select {
		case err := <-retErr:
			if err == nil {
				done = true
				break
			}
			panic(err)
		case userTask := <-ret3:
			testRes3 = append(testRes3, userTask)
		}
	}
	assert.Len(t, testRes2, 2)
	assert.Len(t, testRes3, 1)
}
