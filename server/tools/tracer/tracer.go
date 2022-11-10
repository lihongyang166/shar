package tracer

import (
	"fmt"
	"github.com/nats-io/nats.go"
	"gitlab.com/shar-workflow/shar/common"
	"gitlab.com/shar-workflow/shar/model"
	"google.golang.org/protobuf/proto"
	"strings"
)

// Trace sets a consumer onto the workflow messages and outputs them to the console
func Trace(natsURL string) *nats.Subscription {
	nc, _ := nats.Connect(natsURL)
	sub, err := nc.Subscribe("WORKFLOW.>", func(msg *nats.Msg) {
		if strings.HasPrefix(msg.Subject, "WORKFLOW.default.State.") {
			d := &model.WorkflowState{}
			err := proto.Unmarshal(msg.Data, d)
			if err != nil {
				panic(err)
			}
			fmt.Println(msg.Subject, d.State, last4(d.WorkflowInstanceId), "T:"+last4(common.TrackingID(d.Id).ID()), "P:"+last4(common.TrackingID(d.Id).ParentID()), d.ElementType, d.ElementId)
		} else {
			fmt.Println(msg.Subject)
		}
	})
	if err != nil {
		panic(err)
	}
	return sub
}

func last4(s string) string {
	if len(s) < 4 {
		return ""
	}
	return s[len(s)-4:]
}
