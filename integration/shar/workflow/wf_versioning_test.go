package workflow

import (
	"context"
	"fmt"
	"gitlab.com/shar-workflow/shar/client/task"
	support "gitlab.com/shar-workflow/shar/internal/integration-support"
	"os"
	"testing"

	"github.com/nats-io/nats.go"
	"github.com/segmentio/ksuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gitlab.com/shar-workflow/shar/client"
	"gitlab.com/shar-workflow/shar/common/namespace"
	"gitlab.com/shar-workflow/shar/model"
	"gitlab.com/shar-workflow/shar/server/messages"
)

func TestWfVersioning(t *testing.T) {

	// Create a starting context
	ctx := context.Background()

	// Dial shar
	ns := ksuid.New().String()
	cl := client.New(client.WithEphemeralStorage(), client.WithConcurrency(10), client.WithNamespace(ns))
	err := cl.Dial(ctx, tst.NatsURL)
	require.NoError(t, err)

	// Load service task
	d := &wfTeestandlerDef{t: t, finished: make(chan struct{})}
	_, err = support.RegisterTaskYamlFile(ctx, cl, "wf_versioning_SimpleProcess.yaml", d.integrationSimple)
	require.NoError(t, err)

	// Load BPMN workflow
	b, err := os.ReadFile("../../../testdata/simple-workflow.bpmn")
	require.NoError(t, err)
	_, err = cl.LoadBPMNWorkflowFromBytes(ctx, client.LoadWorkflowParams{Name: "SimpleWorkflowTest", WorkflowBPMN: b})
	require.NoError(t, err)

	res, err := cl.ListWorkflows(ctx)
	require.NoError(t, err)
	assert.Equal(t, int32(1), res[0].Version)
	oldWfId, err := cl.LoadBPMNWorkflowFromBytes(ctx, client.LoadWorkflowParams{Name: "SimpleWorkflowTest", WorkflowBPMN: b})
	require.NoError(t, err)
	res2, err := cl.ListWorkflows(ctx)
	require.NoError(t, err)
	t.Log(len(res2))
	assert.Equal(t, int32(1), res2[0].Version)
	nc, err := nats.Connect(tst.NatsURL)
	require.NoError(t, err)
	js, err := nc.JetStream()
	require.NoError(t, err)
	kv, err := js.KeyValue(namespace.PrefixWith(ns, messages.KvDefinition))
	require.NoError(t, err)
	keys, err := kv.Keys()
	require.NoError(t, err)
	assert.Equal(t, 1, len(keys))

	// Load changed BPMN workflow
	b, err = os.ReadFile("../../../testdata/simple-workflow-changed.bpmn")
	require.NoError(t, err)
	newWfId, err := cl.LoadBPMNWorkflowFromBytes(ctx, client.LoadWorkflowParams{Name: "SimpleWorkflowTest", WorkflowBPMN: b})
	require.NoError(t, err)
	res3, err := cl.ListWorkflows(ctx)
	require.NoError(t, err)
	t.Log(len(res2))
	assert.Equal(t, int32(2), res3[0].Version)
	fmt.Println("YAY")
	vers, err := cl.GetWorkflowVersions(ctx, "SimpleWorkflowTest")
	require.NoError(t, err)
	assert.Len(t, vers, 2)
	assert.Equal(t, int32(1), vers[0].Number)
	assert.Equal(t, int32(2), vers[1].Number)
	assert.Equal(t, oldWfId, vers[0].Id)
	assert.Equal(t, newWfId, vers[1].Id)
	keys, err = kv.Keys()
	require.NoError(t, err)
	assert.Equal(t, 2, len(keys))
	tst.AssertCleanKV(ns, t, tst.Cooldown)
}

type wfTeestandlerDef struct {
	t        *testing.T
	finished chan struct{}
}

func (d *wfTeestandlerDef) integrationSimple(_ context.Context, _ task.JobClient, vars model.Vars) (model.Vars, error) {
	fmt.Println("Hi")
	carried, err := vars.GetInt64("carried")
	require.NoError(d.t, err)
	assert.Equal(d.t, int64(32768), carried)
	localVar, err := vars.GetInt64("localVar")
	require.NoError(d.t, err)
	assert.Equal(d.t, int64(42), localVar)
	vars.SetBool("Success", true)
	return vars, nil
}
