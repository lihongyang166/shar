package integration

import (
	"context"
	"encoding/json"
	"fmt"
	"gitlab.com/shar-workflow/shar/cli/commands"
	integ "gitlab.com/shar-workflow/shar/internal/integration-support"
	"strings"
	"testing"
)

// Result contains the streams to output CLI execution.
type Result struct {
	Out *strings.Builder
	Err *strings.Builder
}

// ExecTst executes a CLI command and returns a struct.
func ExecTst[rt any](t *testing.T, tst *integ.Integration, line string) (rt, error) { // nolint
	ty := new(rt)
	idx := strings.Index(line, " ")
	line2 := line[0:idx] + " --json --server " + strings.Split(tst.NatsURL, "nats://")[1] + line[idx:]
	r, err := Exec(line2)
	if err != nil {
		return *ty, fmt.Errorf("execute test function: %w", err)
	}
	ret := r.Out.String()
	if err := json.Unmarshal([]byte(ret), ty); err != nil {
		return *ty, fmt.Errorf("marshal json: %w", err)
	}
	return *ty, nil
}

// Exec executes a CLI command.
func Exec(line string) (*Result, error) {
	//fmt.Println("$> " + line)
	ctx := context.Background()
	c := commands.RootCmd
	args := strings.Split(line, " ")
	res := &Result{
		Out: new(strings.Builder),
		Err: new(strings.Builder),
	}
	c.SetArgs(args[1:])
	c.SetContext(ctx)
	c.SetErr(res.Err)
	c.SetOut(res.Out)
	err := c.Execute()
	if err != nil {
		return nil, fmt.Errorf("CLI execution failed: %w", err)
	}
	return res, nil
}
