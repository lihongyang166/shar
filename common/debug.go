package common

import (
	"context"
	"fmt"
	"github.com/nats-io/nats.go"
	"gitlab.com/shar-workflow/shar/common/telemetry"
	"go.opentelemetry.io/contrib/propagators/autoprop"
)

func CheckNatsTelemetry(msg *nats.Msg) {
	carrier := telemetry.NewNatsMsgCarrier(msg)
	prop := autoprop.NewTextMapPropagator()
	prop.Inject(context.Background(), carrier)
	tp := carrier.Get("traceparent")
	if tp == "" {
		fmt.Println("###### NO NATS MESSAGE TRACE")
	} else {
		fmt.Println("******", tp)
	}
}

func CheckCtxTelemetry(ctx context.Context) {
	msg := &nats.Msg{
		Header: make(nats.Header),
	}
	carrier := telemetry.NewNatsMsgCarrier(msg)
	prop := autoprop.NewTextMapPropagator()
	prop.Inject(context.Background(), carrier)
	tp := carrier.Get("traceparent")
	if tp == "" {
		fmt.Println("###### NO CTX MESSAGE TRACE")
	} else {
		fmt.Println("******", tp)
	}
}
