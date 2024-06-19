package workflow

import (
	"context"
	errors2 "errors"
	"fmt"
	"github.com/nats-io/nats.go/jetstream"
	"gitlab.com/shar-workflow/shar/common"
	"gitlab.com/shar-workflow/shar/common/logx"
	"gitlab.com/shar-workflow/shar/common/subj"
	"gitlab.com/shar-workflow/shar/model"
	"gitlab.com/shar-workflow/shar/server/errors"
	"gitlab.com/shar-workflow/shar/server/messages"
	"gitlab.com/shar-workflow/shar/server/vars"
	"google.golang.org/protobuf/proto"
	"log/slog"
	"strings"
)

func (s *Engine) processGatewayActivation(ctx context.Context) error {
	err := common.Process(ctx, s.natsService.Js, "WORKFLOW", "gatewayActivate", s.closing, subj.NS(messages.WorkflowJobGatewayTaskActivate, "*"), "GatewayActivateConsumer", s.concurrency, s.receiveMiddleware, func(ctx context.Context, log *slog.Logger, msg jetstream.Msg) (bool, error) {
		ns := subj.GetNS(ctx)
		nsKVs, err := s.natsService.KvsFor(ctx, ns)
		if err != nil {
			return false, fmt.Errorf("get KVs for ns %s: %w", ns, err)
		}

		var job model.WorkflowState
		if err := proto.Unmarshal(msg.Data(), &job); err != nil {
			return false, fmt.Errorf("unmarshal completed gateway activation state: %w", err)
		}
		if _, _, err := s.operations.HasValidProcess(ctx, job.ProcessInstanceId, job.ExecutionId); errors2.Is(err, errors.ErrExecutionNotFound) || errors2.Is(err, errors.ErrProcessInstanceNotFound) {
			log := logx.FromContext(ctx)
			log.Log(ctx, slog.LevelInfo, "processCompletedJobs aborted due to a missing process")
			return true, nil
		} else if err != nil {
			return false, err
		}
		gwIID, _, _ := s.GetGatewayInstanceID(&job)
		job.Id = common.TrackingID(job.Id).Push(gwIID)
		gw := &model.Gateway{}
		if err := common.LoadObj(ctx, nsKVs.WfGateway, gwIID, gw); errors2.Is(err, jetstream.ErrKeyNotFound) {
			// create a new gateway job
			gw = &model.Gateway{
				MetExpectations: make(map[string]string),
				Vars:            [][]byte{job.Vars},
				Visits:          0,
			}
			if err := common.SaveObj(ctx, nsKVs.Job, gwIID, &job); err != nil {
				return false, fmt.Errorf("%s failed to save job to KV: %w", errors.Fn(), err)
			}
			if err := common.SaveObj(ctx, nsKVs.WfGateway, gwIID, gw); err != nil {
				return false, fmt.Errorf("%s failed to save gateway to KV: %w", errors.Fn(), err)
			}
			if err := s.operations.PublishWorkflowState(ctx, messages.WorkflowJobGatewayTaskExecute, &job); err != nil {
				return false, fmt.Errorf("%s failed to execute gateway to KV: %w", errors.Fn(), err)
			}
		} else if err != nil {
			return false, fmt.Errorf("%s could not load gateway information: %w", errors.Fn(), err)
		} else if err := s.operations.PublishWorkflowState(ctx, messages.WorkflowJobGatewayTaskReEnter, &job); err != nil {
			return false, fmt.Errorf("%s failed to execute gateway to KV: %w", errors.Fn(), err)
		}
		return true, nil
	}, s.operations.SignalFatalError)
	if err != nil {
		return fmt.Errorf("initialize gateway activation listener: %w", err)
	}
	return nil
}

func (s *Engine) processGatewayExecute(ctx context.Context) error {
	if err := common.Process(ctx, s.natsService.Js, "WORKFLOW", "gatewayExecute", s.closing, subj.NS(messages.WorkflowJobGatewayTaskExecute, "*"), "GatewayExecuteConsumer", s.concurrency, s.receiveMiddleware, s.gatewayExecProcessor, s.operations.SignalFatalError); err != nil {
		return fmt.Errorf("start process launch processor: %w", err)
	}
	if err := common.Process(ctx, s.natsService.Js, "WORKFLOW", "gatewayReEnter", s.closing, subj.NS(messages.WorkflowJobGatewayTaskReEnter, "*"), "GatewayReEnterConsumer", s.concurrency, s.receiveMiddleware, s.gatewayExecProcessor, s.operations.SignalFatalError); err != nil {
		return fmt.Errorf("start process launch processor: %w", err)
	}
	return nil
}

func (s *Engine) gatewayExecProcessor(ctx context.Context, _ *slog.Logger, msg jetstream.Msg) (bool, error) {
	var job model.WorkflowState
	if err := proto.Unmarshal(msg.Data(), &job); err != nil {
		return false, fmt.Errorf("unmarshal during process launch: %w", err)
	}
	if _, _, err := s.operations.HasValidProcess(ctx, job.ProcessInstanceId, job.ExecutionId); errors2.Is(err, errors.ErrExecutionNotFound) || errors2.Is(err, errors.ErrProcessInstanceNotFound) {
		log := logx.FromContext(ctx)
		log.Log(ctx, slog.LevelInfo, "processLaunch aborted due to a missing process")
		return true, err
	} else if err != nil {
		return false, err
	}
	// Gateway logic
	wf, err := s.operations.GetWorkflow(ctx, job.WorkflowId)
	if err != nil {
		return true, &errors.ErrWorkflowFatal{Err: fmt.Errorf("process gateway execute failed: %w", err)}
	}
	els := make(map[string]*model.Element)
	common.IndexProcessElements(wf.Process[job.ProcessName].Elements, els)
	el := els[job.ElementId]

	gatewayIID, route, err := s.GetGatewayInstanceID(&job)
	if err != nil {
		return false, fmt.Errorf("%s failed to find gateway instance ID: %w", errors.Fn(), err)
	}
	job.Execute = &gatewayIID

	ns := subj.GetNS(ctx)
	nsKVs, err := s.natsService.KvsFor(ctx, ns)
	if err != nil {
		return false, fmt.Errorf("get KVs for ns %s: %w", ns, err)
	}

	gw := &model.Gateway{}
	err = common.UpdateObj(ctx, nsKVs.WfGateway, gatewayIID, gw, func(g *model.Gateway) (*model.Gateway, error) {
		if g.MetExpectations == nil {
			g.MetExpectations = make(map[string]string)
		}
		// TODO: This could be problematic in the case of an error after this line and needs splitting to ensure this value does not modified unchecked
		g.MetExpectations[route] = ""
		g.Vars = append(g.Vars, job.Vars)
		g.Visits++
		return g, nil
	})
	if err != nil {
		return false, fmt.Errorf("save gateway instance metadata: %w", err)
	}

	switch el.Gateway.Type {
	case model.GatewayType_exclusive:
		nv, err := s.mergeGatewayVars(ctx, gw)
		if err != nil {
			return false, fmt.Errorf("merge gateway variables: %w", &errors.ErrWorkflowFatal{Err: err})
		}
		job.Vars = nv
		if err := s.completeGateway(ctx, &job); err != nil {
			return false, err
		}
	case model.GatewayType_inclusive, model.GatewayType_parallel:
		nv, err := s.mergeGatewayVars(ctx, gw)
		if err != nil {
			return false, fmt.Errorf("merge gateway variables: %w", &errors.ErrWorkflowFatal{Err: err})
		}

		if len(gw.MetExpectations) >= len(job.GatewayExpectations[gatewayIID].ExpectedPaths) {
			job.Vars = nv
			if err := s.completeGateway(ctx, &job); err != nil {
				return false, err
			}
		} else {
			if err := s.operations.PublishWorkflowState(ctx, messages.WorkflowJobGatewayTaskAbort, &job); err != nil {
				return false, err
			}
		}
	}
	return true, nil
}

// GetGatewayInstance - returns a gateway instance from the KV store.
func (s *Engine) GetGatewayInstance(ctx context.Context, gatewayInstanceID string) (*model.Gateway, error) {
	ns := subj.GetNS(ctx)
	nsKVs, err := s.natsService.KvsFor(ctx, ns)
	if err != nil {
		return nil, fmt.Errorf("get KVs for ns %s: %w", ns, err)
	}

	gw := &model.Gateway{}
	err = common.LoadObj(ctx, nsKVs.WfGateway, gatewayInstanceID, gw)
	if err != nil {
		return nil, fmt.Errorf("get gateway instance failed to get gateway instance from KV: %w", err)
	}
	return gw, nil
}

// GetGatewayInstanceID - returns a gateawy instance ID and a satisfying route to that gateway.
func (s *Engine) GetGatewayInstanceID(state *model.WorkflowState) (string, string, error) {
	var gatewayIID string
	var route string
	if satisfiesGatewayExpectation, ok := state.SatisfiesGatewayExpectation[state.ElementId]; ok {
		r := satisfiesGatewayExpectation.InstanceTracking[len(satisfiesGatewayExpectation.InstanceTracking)-1]
		parts := strings.Split(r, ",")
		gatewayIID = parts[0]
		route = parts[1]
		return gatewayIID, route, nil
	}
	return "", "", fmt.Errorf("discover gateway instance ID: %w", errors.ErrGatewayInstanceNotFound)
}

func (s *Engine) mergeGatewayVars(ctx context.Context, gw *model.Gateway) ([]byte, error) {
	if len(gw.Vars) == 1 {
		return gw.Vars[0], nil
	}
	base, err := vars.Decode(ctx, gw.Vars[0])
	if err != nil {
		return nil, fmt.Errorf("merge gateway vars failed to decode base variables: %w", err)
	}
	ret, err := vars.Decode(ctx, gw.Vars[0])
	if err != nil {
		return nil, fmt.Errorf("merge gateway vars failed to decode initial variables: %w", err)
	}
	for i := 1; i < len(gw.Vars); i++ {
		v2, err := vars.Decode(ctx, gw.Vars[i])
		if err != nil {
			return nil, fmt.Errorf("merge gateway vars failed to decode variable set %d: %w", i, err)
		}
		for k, v := range v2 {
			bv, ok := base[k]
			if !ok || bv != v {
				ret[k] = v
			}
		}
	}
	retb, err := vars.Encode(ctx, ret)
	if err != nil {
		return nil, fmt.Errorf("merge gateway vars failed to encode variable set: %w", err)
	}
	return retb, nil
}

// TODO: make resilient through message
func (s *Engine) completeGateway(ctx context.Context, job *model.WorkflowState) error {
	ns := subj.GetNS(ctx)
	nsKVs, err := s.natsService.KvsFor(ctx, ns)
	if err != nil {
		return fmt.Errorf("get KVs for ns %s: %w", ns, err)
	}

	// Record that we have closed this gateway.
	if err := common.UpdateObj(ctx, nsKVs.WfProcessInstance, job.ProcessInstanceId, &model.ProcessInstance{}, func(v *model.ProcessInstance) (*model.ProcessInstance, error) {
		if v.GatewayComplete == nil {
			v.GatewayComplete = make(map[string]bool)
		}
		v.GatewayComplete[*job.Execute] = true
		return v, nil
	}); err != nil {
		return fmt.Errorf("%s failed to update gateway: %w", errors.Fn(), err)
	}
	if err := s.operations.PublishWorkflowState(ctx, messages.WorkflowJobGatewayTaskComplete, job); err != nil {
		return err
	}
	if err := common.Delete(ctx, nsKVs.WfGateway, *job.Execute); err != nil {
		return fmt.Errorf("complete gateway failed with: %w", err)
	}
	return nil
}
