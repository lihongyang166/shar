package telemetry

import (
	"context"
	"fmt"
	"github.com/nats-io/nats.go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"gitlab.com/shar-workflow/shar/model"
	"go.opentelemetry.io/otel/exporters/stdout/stdouttrace"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"
	"testing"
)

/*
	These tests hope to cover the following scenarios:
	1. The client has open telemetry, the SHAR server does not, and the telemetry server expects spans.
		In this scenario the client passes telemetry using the nats message serializer.
		SHAR server converts that to a set of context values to pass round.
		Emitted states contain the telemetry values.
	2. The client has open telemetry, the SHAR server has too, and the telemetry server expects spans.
		In this scenario the client passes telemetry using the nats message serializer.
		SHAR server converts that using the open telemetry context serializer, and adds its own spans.
		Emitted states contain the telemetry values for the SHAR parent spans, but the telemetry server
        should use the ID of the WorkflowState.

	Either way, the client should receive its telemetry parameters back when a service task is called.
*/

func TestClientTelemetryToSharServerTelemetry(t *testing.T) {
	testClientToSharServerWithTelemetry(t, true, true)
}

func TestClientNoTelemetryToSharServerTelemetry(t *testing.T) {
	testClientToSharServerWithTelemetry(t, false, true)
}

func TestClientTelemetryToSharServeWithNewTraceIdrNoTelemetry(t *testing.T) {
	testClientToSharServerWithTelemetry(t, true, false)
}

func TestClientNoTelemetryToSharServerNoTelemetry(t *testing.T) {
	testClientToSharServerWithTelemetry(t, false, false)
}

type mockTelemetryCapable struct {
	telemetryConfig Config
}

func (c *mockTelemetryCapable) GetTelemetryConfig() Config {
	return c.telemetryConfig
}

// testClientToSharServerWithTelemetry emulates communication with telemetry across the platform;
// Initially, the mock client calls the server using middleware with a NATS message optionally loaded with telemetry
// The mock server extracts the client telemetry payload using the API middleware, and attaches it to a WorkflowState proto.
// The mock telemetry server receives the message, and checks it.
// Finally, a mock service task host receives the telemetry an optionally uses it for its call context.
func testClientToSharServerWithTelemetry(t *testing.T, clientEnabled bool, serverEnabled bool) {

	// A client
	clientCfg := Config{Enabled: clientEnabled}
	clientSendMiddleware := SendMessageTelemetry(clientCfg)
	clientReceiveMiddleware := ReceiveMessageTelemetry(clientCfg)

	// A server
	serverCfg := Config{Enabled: serverEnabled}

	serverSendMiddleware := SendServerMessageTelemetry(serverCfg)
	serverReceiveMiddleware := ReceiveAPIMessageTelemetry(serverCfg)

	exporter, err := stdouttrace.New(stdouttrace.WithPrettyPrint())
	require.NoError(t, err, "failed to create stdouttrace exporter")
	batchSpanProcessor := sdktrace.NewBatchSpanProcessor(exporter)
	traceProvider := sdktrace.NewTracerProvider(
		sdktrace.WithSampler(sdktrace.AlwaysSample()),
		sdktrace.WithSpanProcessor(batchSpanProcessor),
	)

	msg := nats.NewMsg("test-subject")

	var clientCtx = context.Background()
	var clientSpan trace.Span
	var clientSpanContext trace.SpanContext
	if clientCfg.Enabled {
		clientCtx, clientSpan = traceProvider.Tracer("client-trace").Start(clientCtx, "client-span")
		defer clientSpan.End()
		clientSpanContext = trace.SpanContextFromContext(clientCtx)
	}
	err1 := clientSendMiddleware(clientCtx, msg)
	require.NoError(t, err1)

	fmt.Printf("%+v\n", msg)

	// SHAR Server Side Actions
	var svrCtx = context.Background()
	svrCtx, err = serverReceiveMiddleware(svrCtx, msg)
	require.NoError(t, err)

	var sp2 trace.Span
	var serverSpanCtx trace.SpanContext

	if serverCfg.Enabled {
		svrCtx, sp2 = traceProvider.Tracer("server-trace").Start(svrCtx, "server-span")
		defer sp2.End()

		serverSpanCtx = trace.SpanContextFromContext(svrCtx)
		assert.True(t, serverSpanCtx.HasTraceID())
		assert.True(t, serverSpanCtx.HasSpanID())
	}

	if clientCfg.Enabled {
		var carriedSpanId, carriedTraceId string
		if serverEnabled {
			carriedSpanId = serverSpanCtx.SpanID().String()
			carriedTraceId = serverSpanCtx.TraceID().String()
			assert.NotEqual(t, clientSpanContext.SpanID().String(), carriedSpanId)
		} else {
			carriedTraceId, carriedSpanId = GetTraceparentTraceAndSpan(svrCtx.Value("traceparent").(string))
			assert.Equal(t, clientSpanContext.SpanID().String(), carriedSpanId)
		}
		assert.Equal(t, clientSpanContext.TraceID().String(), carriedTraceId)
	}

	wfState := &model.WorkflowState{}
	CtxToWfState(svrCtx, &Config{Enabled: serverEnabled}, wfState)

	assert.NotEqual(t, wfState.TraceParent[3:35], "00000000000000000000000000000000")

	// SHAR Telemetry
	if serverEnabled {
		assert.Contains(t, wfState.TraceParent, serverSpanCtx.SpanID().String())
		assert.Contains(t, wfState.TraceParent, serverSpanCtx.TraceID().String())
	} else {
		//assert.Contains(t, wfState.TraceParent, serverSpanCtx.TraceID().String())
		if clientEnabled {
			fmt.Println(clientSpanContext)
			assert.Equal(t, wfState.TraceParent[36:52], clientSpanContext.SpanID().String())
		} else {
			assert.Equal(t, wfState.TraceParent[36:52], "1000000000000001")
		}
	}

	// Server to client service task communication
	msg2 := nats.NewMsg("server-to-client")
	err = serverSendMiddleware(svrCtx, msg2)
	require.NoError(t, err)

	// Client service task
	stCtx := context.Background()
	stCtx, err = clientReceiveMiddleware(stCtx, msg2)
	require.NoError(t, err)
	if clientEnabled {
		newSpanCtx := trace.SpanContextFromContext(stCtx)
		assert.True(t, newSpanCtx.HasTraceID())
		assert.True(t, newSpanCtx.HasSpanID())
		assert.Equal(t, clientSpanContext.TraceID(), newSpanCtx.TraceID())
		if serverEnabled {
			assert.NotEqual(t, clientSpanContext.SpanID(), newSpanCtx.SpanID())
		} else {
			assert.Equal(t, clientSpanContext.SpanID(), newSpanCtx.SpanID())
		}
	}
}
