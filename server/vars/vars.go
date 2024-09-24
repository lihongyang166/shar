package vars

import (
	"context"
	"fmt"
	"gitlab.com/shar-workflow/shar/common/expression"
	model2 "gitlab.com/shar-workflow/shar/internal/model"
	"gitlab.com/shar-workflow/shar/model"
	"gitlab.com/shar-workflow/shar/server/errors"
)

// InputVars returns a set of variables matching an input requirement after transformation through expressions contained in an element.
func InputVars(ctx context.Context, eng expression.Engine, oldVarsBin []byte, newVarsBin *[]byte, el *model.Element) error {
	localVars := model2.NewServerVars()
	if el.InputTransform != nil {
		processVars := model2.NewServerVars()
		err := processVars.Decode(ctx, oldVarsBin)
		if err != nil {
			return fmt.Errorf("decode old input variables: %w", err)
		}
		for k, v := range el.InputTransform {
			res, err := expression.EvalAny(ctx, eng, v, processVars.Vals)
			if err != nil {
				return fmt.Errorf("expression evalutaion failed: %w", err)
			}
			localVars.Vals[k] = res
		}
		b, err := localVars.Encode(ctx)
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
		localVars := model2.NewServerVars()
		if err := localVars.Decode(ctx, newVarsBin); err != nil {
			return fmt.Errorf("decode new output variables: %w", err)
		}
		processVars := model2.NewServerVars()
		if mergeVarsBin == nil || len(*mergeVarsBin) > 0 {
			err := processVars.Decode(ctx, *mergeVarsBin)
			if err != nil {
				return fmt.Errorf("decode merge output variables: %w", err)
			}
		}
		for k, v := range transform {
			res, err := expression.EvalAny(ctx, eng, v, localVars.Vals)
			if err != nil {
				return fmt.Errorf("evaluate output transform expression: %w", err)
			}
			processVars.Vals[k] = res
		}
		b, err := processVars.Encode(ctx)
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
		vrs := model2.NewServerVars()
		err := vrs.Decode(ctx, state.Vars)
		if err != nil {
			return fmt.Errorf("falied to decode variables to check: %w", err)
		}
		for _, v := range el.OutputTransform {
			list, err := expression.GetVariables(ctx, eng, v)
			if err != nil {
				return fmt.Errorf("get the variables to check from output transform: %w", err)
			}
			for _, i := range list {
				if _, ok := vrs.Vals[i.Name]; !ok {
					return &errors.ErrWorkflowFatal{Err: fmt.Errorf("expected output variable [%s] missing", i)}
				}
			}
		}
	}
	return nil
}
