package simple

import (
	"context"
	"fmt"
	"github.com/segmentio/ksuid"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gitlab.com/shar-workflow/shar/client"
	"gitlab.com/shar-workflow/shar/client/taskutil"
	support "gitlab.com/shar-workflow/shar/integration-support"
	"gitlab.com/shar-workflow/shar/model"
)

func TestSimple(t *testing.T) {
	//type req struct {
	//	reqId string
	//}
	//
	//type res struct {
	//	reqId string
	//}
	//
	//reqChan := make(chan req, 100)
	//resChan := make(chan res, 100)
	//
	//go func() {
	//	fmt.Println("###starting background thread")
	//	for {
	//		rq := <-reqChan
	//		fmt.Println("###received rq with reqId: " + rq.reqId)
	//		resChan <- res{reqId: rq.reqId}
	//	}
	//}()
	//
	//for i := 0; i < 5; i++ {
	//	go func(j int) {
	//		reqId := ksuid.New().String()
	//		fmt.Println("###sending req for go routine " + fmt.Sprint(j) + " with req id " + reqId)
	//
	//		rand.Seed(time.Now().UnixNano())
	//		randI := rand.Intn(5) + 1
	//		time.Sleep(time.Second * time.Duration(randI))
	//
	//		reqChan <- req{reqId: reqId}
	//	outer:
	//		for {
	//			select {
	//			case res := <-resChan:
	//				fmt.Println("###in go routine " + fmt.Sprint(j) + " received res for id " + res.reqId)
	//				break outer
	//			case <-time.After(time.Second * 8):
	//				fmt.Println("### didn't get response after 2s")
	//				break outer
	//			}
	//		}
	//
	//		fmt.Println("###complete go routine " + fmt.Sprint(j))
	//	}(i)
	//}
	//
	//time.Sleep(time.Second * 30)
	t.Parallel()
	// Create a starting context
	ctx := context.Background()

	// Dial shar
	ns := ksuid.New().String()
	cl := client.New(client.WithEphemeralStorage(), client.WithConcurrency(10), client.Experimental_WithNamespace(ns))

	err := cl.Dial(ctx, tst.NatsURL)
	require.NoError(t, err)

	// Register a service task
	d := &testSimpleHandlerDef{t: t, finished: make(chan struct{})}

	err = taskutil.RegisterTaskYamlFile(ctx, cl, "simple_test.yaml", d.integrationSimple)
	require.NoError(t, err)
	err = cl.RegisterProcessComplete("SimpleProcess", d.processEnd)
	require.NoError(t, err)

	// Load BPMN workflow
	b, err := os.ReadFile("../../../testdata/simple-workflow.bpmn")
	require.NoError(t, err)

	_, err = cl.LoadBPMNWorkflowFromBytes(ctx, "SimpleWorkflowTest", b)
	require.NoError(t, err)

	// Launch the workflow
	_, _, err = cl.LaunchProcess(ctx, "SimpleProcess", model.Vars{})
	require.NoError(t, err)
	// Listen for service tasks
	go func() {
		err := cl.Listen(ctx)
		require.NoError(t, err)
	}()
	support.WaitForChan(t, d.finished, 20*time.Second)

	tst.AssertCleanKV(ns, t, 60*time.Second)
}

type testSimpleHandlerDef struct {
	t        *testing.T
	finished chan struct{}
}

func (d *testSimpleHandlerDef) integrationSimple(_ context.Context, _ client.JobClient, vars model.Vars) (model.Vars, error) {
	fmt.Println("Hi")
	assert.Equal(d.t, 32768, vars["carried"].(int))
	assert.Equal(d.t, 42, vars["localVar"].(int))
	vars["Success"] = true
	return vars, nil
}

func (d *testSimpleHandlerDef) processEnd(_ context.Context, _ model.Vars, _ *model.Error, _ model.CancellationState) {
	close(d.finished)
}
