package workflow

import (
	"context"
	"errors"
	"fmt"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
	"gitlab.com/shar-workflow/shar/common"
	"gitlab.com/shar-workflow/shar/common/element"
	"gitlab.com/shar-workflow/shar/common/header"
	"gitlab.com/shar-workflow/shar/common/subj"
	"gitlab.com/shar-workflow/shar/common/task"
	"gitlab.com/shar-workflow/shar/model"
	"gitlab.com/shar-workflow/shar/server/messages"
	"golang.org/x/exp/maps"
	"golang.org/x/exp/slices"
	"google.golang.org/protobuf/proto"
)

// GetTaskSpecUID fetches
func (s *Operations) GetTaskSpecUID(ctx context.Context, name string) (string, error) {
	tskVer, err := s.GetTaskSpecVersions(ctx, name)
	if err != nil {
		return "", fmt.Errorf("opening task spec versions: %w", err)
	}

	return tskVer.Id[len(tskVer.Id)-1], nil
}

// GetTaskSpecVersions fetches the versions of a given task spec name
func (s *Operations) GetTaskSpecVersions(ctx context.Context, name string) (*model.TaskSpecVersions, error) {
	ns := subj.GetNS(ctx)
	nsKVs, err := s.natsService.KvsFor(ctx, ns)
	if err != nil {
		return nil, fmt.Errorf("get KVs for ns %s: %w", ns, err)
	}

	tskVer := &model.TaskSpecVersions{}
	if err := common.LoadObj(ctx, nsKVs.WfTaskSpecVer, name, tskVer); err != nil {
		return nil, fmt.Errorf("opening task spec versions: %w", err)
	}
	return tskVer, nil
}

// PutTaskSpec writes a task spec to the database.
func (s *Operations) PutTaskSpec(ctx context.Context, spec *model.TaskSpec) (string, error) {
	ns := subj.GetNS(ctx)
	nsKVs, err := s.natsService.KvsFor(ctx, ns)
	if err != nil {
		return "", fmt.Errorf("get KVs for ns %s: %w", ns, err)
	}

	uid, err := task.CreateUID(spec)
	if err != nil {
		return "", fmt.Errorf("put task spec: hash task: %w", err)
	}
	spec.Metadata.Uid = uid
	if err := s.ensureServiceTaskConsumer(ctx, uid); err != nil {
		return "", fmt.Errorf("ensure consumer for service task %s:%w", uid, err)
	}

	if err := common.SaveObj(ctx, nsKVs.WfTaskSpec, spec.Metadata.Uid, spec); err != nil {
		return "", fmt.Errorf("saving task spec: %w", err)
	}
	vers := &model.TaskSpecVersions{}
	if err := common.UpdateObj(ctx, nsKVs.WfTaskSpecVer, spec.Metadata.Type, vers, func(v *model.TaskSpecVersions) (*model.TaskSpecVersions, error) {
		v.Id = append(v.Id, uid)
		subj := messages.WorkflowSystemTaskCreate
		if len(v.Id) == 0 {
			subj = messages.WorkflowSystemTaskUpdate
		}
		msg := nats.NewMsg(subj)
		b, err := proto.Marshal(spec)
		if err != nil {
			return nil, fmt.Errorf("marshal %s system message: %w", subj, err)
		}
		msg.Data = b
		msg.Header.Set(header.SharNamespace, ns)
		if err := s.natsService.Conn.PublishMsg(msg); err != nil {
			return nil, fmt.Errorf("send %s system message: %w", subj, err)
		}
		return v, nil
	}); err != nil {
		return "", fmt.Errorf("saving task spec version: %w", err)
	}
	return uid, nil
}

// GetTaskSpecByUID fetches a task spec from the database.
func (s *Operations) GetTaskSpecByUID(ctx context.Context, uid string) (*model.TaskSpec, error) {
	ns := subj.GetNS(ctx)
	nsKVs, err := s.natsService.KvsFor(ctx, ns)
	if err != nil {
		return nil, fmt.Errorf("get KVs for ns %s: %w", ns, err)
	}

	spec := &model.TaskSpec{}
	if err := common.LoadObj(ctx, nsKVs.WfTaskSpec, uid, spec); err != nil {
		return nil, fmt.Errorf("loading task spec: %w", err)
	}
	return spec, nil
}

// GetTaskSpecUsageByName produces a report of running and executable places where the task spec is in use.
func (s *Operations) GetTaskSpecUsageByName(ctx context.Context, name string) (*model.TaskSpecUsageReport, error) {
	ns := subj.GetNS(ctx)
	nsKVs, err := s.natsService.KvsFor(ctx, ns)
	if err != nil {
		return nil, fmt.Errorf("getKVs for ns %s: %w", ns, err)
	}

	taskSpecVersions, err := s.GetTaskSpecVersions(ctx, name)
	if err != nil {
		return nil, fmt.Errorf("get task spec versions: %w", err)
	}
	wfKeys, err := nsKVs.Wf.Keys(ctx)
	if err != nil {
		return nil, fmt.Errorf("task spec usage by name get workflow version keys: %w", err)
	}
	rptWf := make(map[string]struct{})
	rptPr := make(map[string]struct{})
	for _, vk := range wfKeys {
		wf := &model.Workflow{}
		err := common.LoadObj(ctx, nsKVs.WfVersion, vk, wf)
		if err != nil {
			return nil, fmt.Errorf("task spec usage by name get workflow")
		}
		for _, pr := range wf.Process {
			for _, el := range pr.Elements {
				if el.Type == element.ServiceTask && el.Version != nil && slices.Contains(taskSpecVersions.Id, *el.Version) {
					rptPr[pr.Id] = struct{}{}
					rptWf[wf.Name] = struct{}{}
				}
			}
		}
	}

	piKeys, err := nsKVs.WfProcessInstance.Keys(ctx)
	if err != nil {
		return nil, fmt.Errorf("task spec usage by name get process instance keys: %w", err)
	}

	rptWfExec := make(map[string]struct{})
	rptPrExec := make(map[string]struct{})

	for _, piKey := range piKeys {
		pi := &model.ProcessInstance{}
		err := common.LoadObj(ctx, nsKVs.WfProcessInstance, piKey, pi)
		if err != nil {
			return nil, fmt.Errorf("task spec usage by name get process instance: %w", err)
		}
		rptWfExec[pi.WorkflowId] = struct{}{}
		rptPrExec[pi.ProcessInstanceId] = struct{}{}
	}

	return &model.TaskSpecUsageReport{
		Workflow:                 maps.Keys(rptWf),
		Process:                  maps.Keys(rptPr),
		ExecutingWorkflow:        maps.Keys(rptWfExec),
		ExecutingProcessInstance: maps.Keys(rptPrExec),
	}, nil
}

// GetExecutableWorkflowIds returns a list of all workflow Ids that contain executable processes
func (s *Operations) GetExecutableWorkflowIds(ctx context.Context) ([]string, error) {
	ns := subj.GetNS(ctx)
	nsKVs, err := s.natsService.KvsFor(ctx, ns)
	if err != nil {
		return nil, fmt.Errorf("GetExecutableWorkflowIds - failed getting KVs for ns %s: %w", ns, err)
	}

	verKeys, err := nsKVs.WfVersion.Keys(ctx)
	if err != nil {
		return nil, fmt.Errorf("get workflow version keys: %w", err)
	}
	res := make([]string, 0, len(verKeys))
	for _, verKey := range verKeys {
		wfv := &model.WorkflowVersions{}
		err := common.LoadObj(ctx, nsKVs.WfVersion, verKey, wfv)
		if err != nil {
			return nil, fmt.Errorf("get workflow version: %w", err)
		}
		res = append(res, wfv.Version[len(wfv.Version)-1].Id)
	}
	return res, nil
}

// GetTaskSpecUsage returns the usage report for a list of task specs.
func (s *Operations) GetTaskSpecUsage(ctx context.Context, uid []string) (*model.TaskSpecUsageReport, error) {
	ns := subj.GetNS(ctx)
	nsKVs, err := s.natsService.KvsFor(ctx, ns)
	if err != nil {
		return nil, fmt.Errorf("get KVs for ns %s: %w", ns, err)
	}

	wfKeys, err := s.GetExecutableWorkflowIds(ctx)
	if !errors.Is(err, jetstream.ErrNoKeysFound) && err != nil {
		return nil, fmt.Errorf("get executable workflows: %w", err)
	}
	rptWf := make(map[string]struct{})
	rptPr := make(map[string]struct{})
	for _, vk := range wfKeys {
		wf := &model.Workflow{}
		err := common.LoadObj(ctx, nsKVs.Wf, vk, wf)
		if err != nil {
			return nil, fmt.Errorf("task spec usage by name get workflow")
		}
		for _, pr := range wf.Process {
			for _, el := range pr.Elements {
				if el.Type == element.ServiceTask && el.Version != nil && slices.Contains(uid, *el.Version) {
					rptPr[pr.Id] = struct{}{}
					rptWf[wf.Name] = struct{}{}
				}
			}
		}
	}

	piKeys, err := nsKVs.WfProcessInstance.Keys(ctx)
	if !errors.Is(err, jetstream.ErrNoKeysFound) && err != nil {
		return nil, fmt.Errorf("task spec usage by name get process instance keys: %w", err)
	}

	rptWfExec := make(map[string]struct{})
	rptPrExec := make(map[string]struct{})

	for _, piKey := range piKeys {
		pi := &model.ProcessInstance{}
		err := common.LoadObj(ctx, nsKVs.WfProcessInstance, piKey, pi)
		if err != nil {
			return nil, fmt.Errorf("task spec usage by name get process instance: %w", err)
		}
		rptWfExec[pi.WorkflowId] = struct{}{}
		rptPrExec[pi.ProcessInstanceId] = struct{}{}
	}

	return &model.TaskSpecUsageReport{
		Workflow:                 maps.Keys(rptWf),
		Process:                  maps.Keys(rptPr),
		ExecutingWorkflow:        maps.Keys(rptWfExec),
		ExecutingProcessInstance: maps.Keys(rptPrExec),
	}, nil
}
