package tracer

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/nats-io/nats.go"
	model2 "gitlab.com/shar-workflow/shar/internal/model"
	"gitlab.com/shar-workflow/shar/model"
	"google.golang.org/protobuf/proto"
	"os"
	"regexp"
	"strings"
)

// OpenDebug represents a running trace.
type OpenDebug struct {
	sub  *nats.Subscription
	file *os.File
}

// Close closes a trace and the underlying connection.
func (o *OpenDebug) Close() {
	err := o.file.Close()
	if err != nil {
		panic(err)
	}
	err = o.sub.Drain()
	if err != nil {
		panic(err)
	}
}

// Debug sets a consumer onto the workflow messages and outputs them to the console
func Debug(natsURL string, filename string) *OpenDebug {
	nc, _ := nats.Connect(natsURL)
	r, _ := regexp.Compile(`^WORKFLOW\.[0-9a-zA-Z]+\.State\..*$`)

	of, err := os.OpenFile(filename, os.O_CREATE+os.O_TRUNC+os.O_WRONLY, 0777)
	if err != nil {
		panic(err)
	}
	sub, err := nc.Subscribe("WORKFLOW.>", func(msg *nats.Msg) {
		if r.MatchString(msg.Subject) {
			debugOutput(msg, of)
		} else {
			fmt.Println(msg.Subject)
		}
	})
	if err != nil {
		panic(err)
	}
	return &OpenDebug{sub: sub, file: of}
}

// OutputState represents the output state of a workflow.
// It embeds the model.WorkflowState struct and extends it with additional fields:
//   - Subject: the subject of the message
//   - VarMap: a map of variables
type OutputState struct {
	model.WorkflowState
	Subject string
	VarMap  map[string]interface{}
}

func debugOutput(msg *nats.Msg, of *os.File) {
	if strings.Contains(msg.Subject, ".State.") {
		st := &OutputState{}
		if err := proto.Unmarshal(msg.Data, st); err != nil {
			panic(err)
		}
		vm := model2.NewServerVars()
		err := vm.Decode(context.Background(), st.Vars)
		if err != nil {
			panic(err)
		}
		st.VarMap = vm.Vals
		st.Vars = []byte{}
		st.Subject = msg.Subject
		b, err := json.Marshal(st)
		if err != nil {
			panic(err)
		}
		if _, err := fmt.Fprintln(of, string(b)); err != nil {
			panic(err)
		}

	}
}
