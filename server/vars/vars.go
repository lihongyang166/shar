package vars

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/shamaton/msgpack"
	"gitlab.com/shar-workflow/shar/common/expression"
	"gitlab.com/shar-workflow/shar/common/logx"
	"gitlab.com/shar-workflow/shar/model"
	"gitlab.com/shar-workflow/shar/server/errors"
)

func msgPackPackedIntToInt(unpacked model.Vars) model.Vars {
	for k, v := range unpacked {
		switch unpacked[k].(type) {
		// case int:
		// 	unpacked[k] = int(v.(int))
		case int8:
			unpacked[k] = int(v.(int8))
		case int16:
			unpacked[k] = int(v.(int16))
		// case int32:
		// 	unpacked[k] = int(v.(int32))
		// case uint:
		// 	unpacked[k] = int(v.(uint))
		case uint8:
			unpacked[k] = int(v.(uint8))
		case uint16:
			unpacked[k] = int(v.(uint16))
		// case uint32:
		// 	unpacked[k] = int(v.(uint32))
		// case uint64:
		// 	unpacked[k] = int(v.(uint64))
		// case int64:
		// 	unpacked[k] = int(v.(int64))
		// case float32:
		// 	unpacked[k] = int(v.(float32))
		// case float64:
		// 	unpacked[k] = int(v.(float64))
		default:
		}
	}
	return unpacked
}

// func init() {
// 	d := msgpack.GetDecoder()
// 	d.UseLooseInterfaceDecoding(true)
// 	msgpack.PutDecoder(d)
// }

// Encode encodes the map of workflow variables into a go binary to be sent across the wire.
func Encode(ctx context.Context, vars model.Vars) (b []byte, err error) {
	if b, err = msgpack.Marshal(vars); err != nil {
		return nil, logx.Err(ctx, "encode vars", &errors.ErrWorkflowFatal{Err: err}, slog.Any("vars", vars))
	}
	return b, nil
}

// Decode decodes a go binary object containing workflow variables.
func Decode(ctx context.Context, vars []byte) (model.Vars, error) {
	// d := msgpack.GetDecoder()
	// d.UseLooseInterfaceDecoding(true)
	// msgpack.PutDecoder(d)

	ret := make(map[string]any)
	if len(vars) == 0 {
		return ret, nil
	}

	if err := msgpack.Unmarshal(vars, &ret); err != nil {
		return nil, logx.Err(ctx, "decode vars", &errors.ErrWorkflowFatal{Err: err}, slog.Any("vars", vars))
	}

	return msgPackPackedIntToInt(ret), nil
}

// InputVars returns a set of variables matching an input requirement after transformation through expressions contained in an element.
func InputVars(ctx context.Context, eng expression.Engine, oldVarsBin []byte, newVarsBin *[]byte, el *model.Element) error {
	localVars := make(map[string]interface{})
	if el.InputTransform != nil {
		processVars, err := Decode(ctx, oldVarsBin)
		if err != nil {
			return fmt.Errorf("decode old input variables: %w", err)
		}
		for k, v := range el.InputTransform {
			res, err := expression.EvalAny(ctx, eng, v, processVars)
			if err != nil {
				return fmt.Errorf("expression evalutaion failed: %w", err)
			}
			localVars[k] = res
		}
		b, err := Encode(ctx, localVars)
		if err != nil {
			return fmt.Errorf("encode transofrmed input variables: %w", err)
		}
		*newVarsBin = b
	}
	return nil
}

// OutputVars merges one variable set into another based upon any expressions contained in an element.
func OutputVars(ctx context.Context, eng expression.Engine, newVarsBin []byte, mergeVarsBin *[]byte, transform map[string]string) error {
	if transform != nil {
		localVars, err := Decode(ctx, newVarsBin)
		if err != nil {
			return fmt.Errorf("decode new output variables: %w", err)
		}
		var processVars map[string]interface{}
		if mergeVarsBin == nil || len(*mergeVarsBin) > 0 {
			pv, err := Decode(ctx, *mergeVarsBin)
			if err != nil {
				return fmt.Errorf("decode merge output variables: %w", err)
			}
			processVars = pv
		} else {
			processVars = make(map[string]interface{})
		}
		for k, v := range transform {
			res, err := expression.EvalAny(ctx, eng, v, localVars)
			if err != nil {
				return fmt.Errorf("evaluate output transform expression: %w", err)
			}
			processVars[k] = res
		}
		b, err := Encode(ctx, processVars)
		if err != nil {
			return fmt.Errorf("encode new output process variables: %w", err)
		}
		*mergeVarsBin = b
	}
	return nil
}

// CheckVars checks for missing variables expected in a result
func CheckVars(ctx context.Context, eng expression.Engine, state *model.WorkflowState, el *model.Element) error {
	if el.OutputTransform != nil {
		vrs, err := Decode(ctx, state.Vars)
		if err != nil {
			return fmt.Errorf("falied to decode variables to check: %w", err)
		}
		for _, v := range el.OutputTransform {
			list, err := expression.GetVariables(ctx, eng, v)
			if err != nil {
				return fmt.Errorf("get the variables to check from output transform: %w", err)
			}
			for _, i := range list {
				if _, ok := vrs[i.Name]; !ok {
					return &errors.ErrWorkflowFatal{Err: fmt.Errorf("expected output variable [%s] missing", i)}
				}
			}
		}
	}
	return nil
}
