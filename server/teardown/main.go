package main

import (
	"context"
	"fmt"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
	"gitlab.com/shar-workflow/shar/server/messages"
	"os"
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

	if err := js.DeleteStream(ctx, "WORKFLOW"); err != nil {
		fmt.Printf("*Not Deleted Stream WORKFLOW: %s\n", err.Error())
	} else {
		fmt.Printf("Deleted stream WORKFLOW\n")
	}
	kvDelete(ctx, js,
		messages.KvUserTask,
		messages.KvInstance,
		messages.KvDefinition,
		messages.KvJob,
		messages.KvTracking,
		messages.KvVersion,
	)

}

func kvDelete(ctx context.Context, js jetstream.JetStream, buckets ...string) {
	for _, v := range buckets {
		if err := js.DeleteKeyValue(ctx, v); err != nil {
			fmt.Printf("*Not Deleted %s: %s\n", v, err.Error())
		} else {
			fmt.Printf("Deleted %s\n", v)
		}
	}
}
