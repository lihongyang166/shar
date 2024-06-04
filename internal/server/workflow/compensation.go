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
	"gitlab.com/shar-workflow/shar/model"
	"gitlab.com/shar-workflow/shar/server/messages"
	"gitlab.com/shar-workflow/shar/server/vars"
	"google.golang.org/protobuf/proto"
	"log/slog"
	"slices"
	"strconv"
	"time"
)

const consumerPrefix = "comp_plan_"

// Compensate is a method of the Engine struct that performs the compensation process for a given workflow state.
// It retrieves the necessary information and history from the workflow and processes it to determine the compensation steps.
// It then creates a compensation plan and publishes each step to a designated subject for further processing.
// Finally, it updates the state of the workflow to indicate that the compensation is in progress.
// It returns an error if any step of the compensation process encounters an issue.
func (s *Engine) Compensate(ctx context.Context, state *model.WorkflowState) error {
	ns := subj.GetNS(ctx)
	kvs, err := s.natsService.KvsFor(ctx, ns)
	if err != nil {
		return fmt.Errorf("get kvs for compensate: %w", err)
	}
	wf, err := s.operations.GetWorkflow(ctx, state.WorkflowId)
	if err != nil {
		return fmt.Errorf("get workflow: %w", err)
	}
	pr := wf.Process[state.ProcessName]
	els := make(map[string]*model.Element)
	common.IndexProcessElements(pr.Elements, els)
	hist := make(chan *model.ProcessHistoryEntry)
	errs := make(chan error, 1)
	//ids := make([]string, 0)
	go s.operations.GetProcessHistory(ctx, state.ProcessInstanceId, hist, errs)
	stateID := common.TrackingID(state.Id)
	planPrefix := subj.NS("WORKFLOW.%s.Compensate.", ns)
	planSubject := planPrefix + stateID.ID()
	if _, err := s.natsService.Js.CreateConsumer(ctx, "WORKFLOW", jetstream.ConsumerConfig{
		Name:              "",
		Durable:           consumerPrefix + stateID.ID(),
		Description:       "compensation plan for " + stateID.ID(),
		DeliverPolicy:     0,
		OptStartSeq:       0,
		OptStartTime:      nil,
		AckPolicy:         jetstream.AckExplicitPolicy,
		AckWait:           time.Second * 60,
		FilterSubject:     planSubject,
		MaxAckPending:     1,
		MaxRequestBatch:   1,
		InactiveThreshold: 0, // May need changing to ensure eventual cleanup
		MemoryStorage:     s.natsService.StorageType == jetstream.MemoryStorage,
	}); err != nil {
		return fmt.Errorf("create compensation consumer: %w", err)
	}

	type step struct {
		UnixTime int64
		Item     string
	}

	forCompensation := make([]step, 0)

	exit := false
	for {
		select {
		case entry := <-hist:
			if entry == nil {
				exit = true
				break
			}
			// Don't include and history output as a result of compensation
			// Only include history items that are compensable
			if entry.Compensating {
				continue
			}
			entryId := common.TrackingID(entry.Id)
			if entry.ItemType == model.ProcessHistoryType_jobExecute && els[*entry.ElementId].CompensateWith != nil {
				// check if the activity completed
				completedId := fmt.Sprintf("%s.%s.%d", *entry.ProcessInstanceId, entryId.ParentID(), int(model.ProcessHistoryType_activityComplete))
				if _, err := kvs.WfHistory.Get(ctx, completedId); errors.Is(err, jetstream.ErrKeyNotFound) {
					continue
				}
				forCompensation = append(forCompensation, step{UnixTime: entry.UnixTimeNano, Item: fmt.Sprintf("%s.%s.%d", *entry.ProcessInstanceId, entryId.ID(), int(entry.ItemType))})
			}

		case err := <-errs:
			if err == nil {
				exit = true
				break
			}
			return fmt.Errorf("get process history: %w", err)
		}
		if exit {
			break
		}
	}

	slices.SortFunc(forCompensation, func(a, b step) int {
		return int(b.UnixTime - a.UnixTime)
	})
	length := len(forCompensation)
	for i, v := range forCompensation {
		msg := nats.NewMsg(planSubject)
		msg.Header.Set(header.SharNamespace, ns)
		msg.Header.Set("Step", strconv.Itoa(i))
		msg.Header.Set("Steps", strconv.Itoa(length))
		msg.Data = []byte(v.Item)
		if err := s.natsService.Conn.PublishMsg(msg); err != nil {
			return fmt.Errorf("publish compensation plan entry: %w", err)
		}
	}

	state.State = model.CancellationState_compensating

	if err := s.operations.PublishWorkflowState(ctx, messages.WorkflowProcessCompensate, state); err != nil {
		return fmt.Errorf("publish compensation state: %w", err)
	}
	return nil
}

func (s *Engine) processProcessCompensate(ctx context.Context) error {
	err := common.Process(ctx, s.natsService.Js, "WORKFLOW", "processCompensate", s.closing, subj.NS(messages.WorkflowProcessCompensate, "*"), "ProcessCompensateConsumer", s.concurrency, s.receiveMiddleware, func(ctx context.Context, log *slog.Logger, msg jetstream.Msg) (bool, error) {
		state := &model.WorkflowState{}
		err := proto.Unmarshal(msg.Data(), state)
		if err != nil {
			return false, fmt.Errorf("unmarshal workflow state: %w", err)
		}
		wf, err := s.operations.GetWorkflow(ctx, state.WorkflowId)
		if err != nil {
			return false, fmt.Errorf("get workflow: %w", err)
		}
		els := make(map[string]*model.Element)
		common.IndexProcessElements(wf.Process[state.ProcessName].Elements, els)
		id := common.TrackingID(state.Id)
		consumer, err := s.natsService.Js.Consumer(ctx, "WORKFLOW", consumerPrefix+id.ID())
		if err != nil {
			return false, fmt.Errorf("get compensation consumer: %w", err)
		}
		cmsgs, err := consumer.Fetch(1)
		if err != nil {
			return false, fmt.Errorf("get compensation plan entry: %w", err)
		}
		cmsg := <-cmsgs.Messages()
		data := cmsg.Data()
		step, err := strconv.Atoi(cmsg.Headers().Get("Step"))
		if err != nil {
			return false, fmt.Errorf("get compensation step: %w", err)
		}
		steps, err := strconv.Atoi(cmsg.Headers().Get("Steps"))
		if err != nil {
			return false, fmt.Errorf("get total compensation steps: %w", err)
		}
		if err := cmsg.Ack(); err != nil {
			return false, fmt.Errorf("ack compensation plan entry: %w", err)
		}
		ns := subj.GetNS(ctx)
		kvs, err := s.natsService.KvsFor(ctx, ns)
		if err != nil {
			return false, fmt.Errorf("get key value stores for namespace: %w", err)
		}
		jobExecuteHistoryID := string(data)
		v, err := kvs.WfHistory.Get(ctx, jobExecuteHistoryID)
		if err != nil {
			return false, fmt.Errorf("get compensation history entry: %w", err)
		}
		jobExe := &model.ProcessHistoryEntry{}
		if err := proto.Unmarshal(v.Value(), jobExe); err != nil {
			return true, fmt.Errorf("unmarshal process history entry: %w", err)
		}
		originalElement := els[*jobExe.ElementId]
		el := els[*originalElement.CompensateWith]
		compensationJob := &model.WorkflowState{
			WorkflowId:        *jobExe.WorkflowId,
			ElementId:         el.Id,
			ElementType:       el.Type,
			Id:                id,
			Execute:           &el.Execute,
			State:             model.CancellationState_compensating,
			Condition:         nil,
			UnixTimeNano:      time.Now().UnixNano(),
			Vars:              state.Vars,
			WorkflowName:      state.WorkflowName,
			ProcessName:       state.ProcessName,
			ProcessInstanceId: state.ProcessInstanceId,
			Compensation: &model.Compensation{
				Step:          int64(step),
				TotalSteps:    int64(steps),
				ForTrackingId: common.TrackingID(jobExe.Id).ID(),
			},
		}
		if jobExe.ExecutionId != nil {
			compensationJob.ExecutionId = *jobExe.ExecutionId
		}
		if el.Version != nil {
			compensationJob.ExecuteVersion = *el.Version
		}

		if err := s.operations.RecordHistoryCompensationCheckpoint(ctx, compensationJob); err != nil {
			return false, fmt.Errorf("record compensation checkpoint: %w", err)
		}
		switch compensationJob.ElementType {
		case element.ServiceTask:
			jobSubject := subj.NS(messages.WorkflowJobServiceTaskExecute, ns) + "." + compensationJob.ExecuteVersion
			if err := s.operations.StartJob(ctx, jobSubject, compensationJob, el, state.Vars); err != nil {
				return false, fmt.Errorf("start job: %w", err)
			}
		}
		//err := s.StartJob(ctx,subj.NS(messages.job))
		return true, nil
	}, nil)
	if err != nil {
		return fmt.Errorf("start compensate processor: %w", err)
	}
	return nil
}

func (s *Engine) compensationJobComplete(ctx context.Context, job *model.WorkflowState) error {
	id := common.TrackingID(job.Id).Pop()
	checkpoint, err := s.operations.GetProcessHistoryItem(ctx, job.ProcessInstanceId, id.ID(), model.ProcessHistoryType_compensationCheckpoint)
	if err != nil {
		return fmt.Errorf("get compensation checkpoint: %w", err)
	}
	wf, err := s.operations.GetWorkflow(ctx, job.WorkflowId)
	if err != nil {
		return fmt.Errorf("get workflow: %w", err)
	}
	els := make(map[string]*model.Element, 0)
	common.IndexProcessElements(wf.Process[job.ProcessName].Elements, els)

	if err := vars.OutputVars(ctx, job.Vars, &checkpoint.Vars, els[job.ElementId].OutputTransform); err != nil {
		return fmt.Errorf("transform output vars: %w", err)
	}

	state := common.CopyWorkflowState(job)
	state.Vars = checkpoint.Vars
	if state.Compensation.Step == state.Compensation.TotalSteps-1 {
		activity, err := s.operations.GetProcessHistoryItem(ctx, state.ProcessInstanceId, id.ID(), model.ProcessHistoryType_activityExecute)
		if err != nil {
			return fmt.Errorf("get compensation history activity entry: %w", err)
		}
		state.State = model.CancellationState_executing
		state.ElementId = *activity.ElementId
		state.ElementType = els[*activity.ElementId].Type
		common.DropStateParams(state)
		state.Id = id

		if err := s.operations.PublishWorkflowState(ctx, messages.WorkflowActivityComplete, state); err != nil {
			return fmt.Errorf("publish compensation activity complete: %w", err)
		}
		if err := s.operations.RecordHistoryActivityComplete(ctx, state); err != nil {
			return fmt.Errorf("record history activity complete for compensation: %w", err)
		}

		if state.ElementType == element.CompensateEndEvent {
			finalState := common.CopyWorkflowState(state)
			el := els[state.ElementId]
			finalState.Id = []string{state.ProcessInstanceId}
			finalState.State = model.CancellationState_completed
			localVars := make([]byte, 0)
			if err := vars.OutputVars(ctx, finalState.Vars, &localVars, el.OutputTransform); err != nil {
				return fmt.Errorf("transform output vars: %w", err)
			}
			finalState.Vars = localVars
			if err := s.operations.PublishWorkflowState(ctx, messages.WorkflowProcessComplete, finalState); err != nil {
				return fmt.Errorf("publish workflow status: %w", err)
			}
		}
	} else {
		state.Id = id
		if err := s.operations.PublishWorkflowState(ctx, messages.WorkflowProcessCompensate, state); err != nil {
			return fmt.Errorf("publish workflow state: %w", err)
		}
		fmt.Println("wait")
	}
	return nil
}
