package tracer

import (
	"context"
	"fmt"
	"github.com/nats-io/nats.go"
	"gitlab.com/shar-workflow/shar/common"
	"gitlab.com/shar-workflow/shar/model"
	"gitlab.com/shar-workflow/shar/server/vars"
	"google.golang.org/protobuf/proto"
	"regexp"
)

// OpenTrace represents a running trace.
type OpenTrace struct {
	sub *nats.Subscription
}

// Close closes a trace and the underlying connection.
func (o *OpenTrace) Close() {
	err := o.sub.Drain()
	if err != nil {
		panic(err)
	}
}

// Trace sets a consumer onto the workflow messages and outputs them to the console
func Trace(natsURL string) *OpenTrace {
	ctx := context.Background()

	nc, _ := nats.Connect("nats://127.0.0.1:4222")
	//nc, _ := nats.Connect(natsURL)
	r, _ := regexp.Compile(`^WORKFLOW\.[0-9a-zA-Z]+\.State\..*$`)
	sub, err := nc.Subscribe("WORKFLOW.>", func(msg *nats.Msg) {
		if r.MatchString(msg.Subject) {
			d := &model.WorkflowState{}
			err := proto.Unmarshal(msg.Data, d)
			if err != nil {
				panic(err)
			}
			dc, err := vars.Decode(ctx, d.Vars)
			var vrs string
			if err != nil {
				vrs = "[error]"
			} else {
				vrs = "["
				for k, v := range dc {
					vrs = vrs + "\"" + k + "\": " + fmt.Sprintf("%+v", v) + ", "
				}
				vrs = vrs + "]"
			}
			fmt.Println(msg.Subject, d.State, last4(d.ExecutionId), "T:"+last4(common.TrackingID(d.Id).ID()), "P:"+last4(common.TrackingID(d.Id).ParentID()), d.ElementType, d.ElementId, vrs)
		} else {
			fmt.Println(msg.Subject)
		}
	})
	if err != nil {
		panic(err)
	}
	return &OpenTrace{sub: sub}
}

func last4(s string) string {
	if len(s) < 4 {
		return ""
	}
	return s[len(s)-4:]
}
