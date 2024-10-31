package main

import (
	"context"
	"fmt"
	"os"

	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
	"gitlab.com/shar-workflow/shar/server/messages"
)

func main() {
	var natsURI string
	if len(os.Args) < 2 {
		natsURI = "nats://127.0.0.1:4222"
	} else {
		natsURI = os.Args[1]
	}
	ctx := context.Background()
	con, err := nats.Connect(natsURI)
	if err != nil {
		panic(err)
	}
	js, err := jetstream.New(con)
	if err != nil {
		panic(err)
	}
	kvStreams := []string{
		"WORKFLOW",
		"WORKFLOW_TELEMETRY",
		"KV_default_" + messages.KvJob,
		"KV_default_" + messages.KvVersion,
		"KV_default_" + messages.KvDefinition,
		"KV_default_" + messages.KvTracking,
		"KV_default_" + messages.KvInstance,
		"KV_default_" + messages.KvExecution,
		"KV_default_" + messages.KvUserTask,
		"KV_default_" + messages.KvOwnerName,
		"KV_default_" + messages.KvOwnerID,
		"KV_default_" + messages.KvClientTaskID,
		"KV_default_" + messages.KvWfName,
		"KV_default_" + messages.KvProcessInstance,
		"KV_default_" + messages.KvGateway,
		"KV_default_" + messages.KvHistory,
		"KV_default_" + messages.KvLock,
		"KV_default_" + messages.KvMessageTypes,
		"KV_default_" + messages.KvTaskSpecVersions,
		"KV_default_" + messages.KvTaskSpec,
		"KV_default_" + messages.KvProcess,
		"KV_default_" + messages.KvMessages,
		"KV_default_" + messages.KvClients,
		"KV_default_" + messages.KvUserTaskState,
		"KV_MsgTx_default_continueMessage",
		"KV_default_WORKFLOW_VARSTATE",
	}
	deletedStreamCount := 0
	errDeletedStream := 0
	for _, v := range kvStreams {
		if err := js.DeleteStream(ctx, v); err != nil {
			fmt.Printf("*Error Deleting Stream %s: %s\n", v, err.Error())
			errDeletedStream++
		} else {
			fmt.Printf("Deleted stream %s\n", v)
			deletedStreamCount++
		}
	}

	fmt.Printf("Could not delete %d streams\n", errDeletedStream)
	fmt.Printf("Deleted %d streams successfully\n", deletedStreamCount)
}
