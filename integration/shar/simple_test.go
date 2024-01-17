package intTest

import (
	"context"
	"fmt"
	"math/rand"
	"os"
	"runtime"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gitlab.com/shar-workflow/shar/client"
	"gitlab.com/shar-workflow/shar/client/taskutil"
	"gitlab.com/shar-workflow/shar/common/namespace"
	support "gitlab.com/shar-workflow/shar/integration-support"
	"gitlab.com/shar-workflow/shar/model"
)

func TestSimple(t *testing.T) {
	tst := &support.Integration{}
	tst.WithTrace = true

	// ### we'd potentially need to modify this so that it just starts a single
	// instance of shar/nats?? each of the concurrently running tests should be
	// properly isolated from each other???
	tst.Setup(t, nil, nil)
	defer tst.Teardown()

	// Create a starting context
	ctx := context.Background()

	// Dial shar
	cl := client.New(client.WithEphemeralStorage(), client.WithConcurrency(10),
		//### can we run two instances of these tests in parallel but have different values for
		// the namespace used here???
		client.Experimental_WithNamespace(namespace.Default),
		// maybe we could drive this via a parallel table test with multiple instances of the test
		// running in separate namespaces running in parallel?
	)

	err := cl.Dial(ctx, tst.NatsURL)
	require.NoError(t, err)

	// Register a service task
	d := &testSimpleHandlerDef{t: t, finished: make(chan struct{})}

	err = taskutil.RegisterTaskYamlFile(ctx, cl, "simple_test.yaml", d.integrationSimple)
	require.NoError(t, err)
	err = cl.RegisterProcessComplete("SimpleProcess", d.processEnd)
	require.NoError(t, err)

	// Load BPMN workflow
	b, err := os.ReadFile("../../testdata/simple-workflow.bpmn")
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

	// ### is the assertion that the kvs used in shar are empty enough to validate that
	// namespaces are isolated enough from each other???
	// we'd need to decide which kvs we should isolate by namespace and which ones
	// should be global and adjust this assert function accordingly...
	tst.AssertCleanKV()

	// ### adding namespace support means we may want to introduce support for
	// metrics partitioned by namespace too? it would probably just be a matter
	// of adding a ns tag to the generated metrics???
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

func (d *testSimpleHandlerDef) processEnd(ctx context.Context, vars model.Vars, wfError *model.Error, state model.CancellationState) {
	close(d.finished)
}

func concurrentMapAccess() {
	var m = make(map[string]string)
	var lock = sync.RWMutex{}

	getCreateIfAbsent := func(k string) string {
		lock.RLock()
		if v, exists := m[k]; !exists {
			lock.RUnlock()
			lock.Lock()
			var v, exists = m[k]
			if !exists {
				m[k] = k
				v = k
			}
			lock.Unlock()
			return v
		} else {
			lock.RUnlock()
			return v
		}
	}

	keys := []string{"a", "b", "c", "d"}

	for i := 0; i < 1000; i++ {
		rand.New(rand.NewSource(time.Now().UnixNano())) //nolint:all
		ki := rand.Intn((len(keys) - 1))                //nolint:all
		fmt.Printf("###ki is: %d", ki)
		go getCreateIfAbsent(keys[ki])
	}

}

func BenchmarkFoo(b *testing.B) {
	for n := 0; n < b.N; n++ {
		concurrentMapAccess()
	}

	goMaxProcs := runtime.GOMAXPROCS(0)
	fmt.Printf("###goMaxProcs: %d", goMaxProcs)
}
