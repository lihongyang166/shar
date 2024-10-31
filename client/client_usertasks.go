package client

import (
	"context"
	"errors"
	"fmt"
	"gitlab.com/shar-workflow/shar/common/subj"
	api2 "gitlab.com/shar-workflow/shar/internal/client/api"
	"gitlab.com/shar-workflow/shar/model"
	"gitlab.com/shar-workflow/shar/server/messages"
	"time"
)

// ErrIndexNotReady indicates that an operation cannot be completed because the index is not ready.
var ErrIndexNotReady = errors.New("index not ready")

// UserTaskQuery represents the criteria for querying user tasks.
type UserTaskQuery struct {
	UserID        string // UserID to return tasks for either provide this or GroupID.
	GroupID       string // GroupID to return tasks for either provide this or UserID.
	IncludeLocked bool   // Include tasks locked to other users.
}

// UserTaskInstance represents a task assigned to a user.
type UserTaskInstance interface {
	ID() string                     // ID - the ID of the task
	Spec() (*model.TaskSpec, error) // Spec - the task specification for the task.
	State() (model.Vars, error)     // State - the current variable state of the task.
	LockedBy() (string, error)      // LockedBy - return the task's owner.
}

type userTaskInstance struct {
	id       string
	spec     *model.TaskSpec
	state    model.Vars
	lockedBy string
}

func (ut *userTaskInstance) ID() string {
	return ut.id
}
func (ut *userTaskInstance) Spec() (*model.TaskSpec, error) {
	return ut.spec, nil
}
func (ut *userTaskInstance) State() (model.Vars, error) {
	return ut.state, nil
}
func (ut *userTaskInstance) LockedBy() (string, error) {
	return ut.lockedBy, nil
}

// openUserTaskOptions are options for opening user tasks
type openUserTaskOptions struct {
	lockDuration time.Duration
}

// OpenUserTaskOpts are functional options for configuring parameters when opening a user task.
type OpenUserTaskOpts func(opts *openUserTaskOptions)

// WithLockDuration sets the lock duration for the user task.
func WithLockDuration(d time.Duration) OpenUserTaskOpts {
	return func(opts *openUserTaskOptions) {
		opts.lockDuration = d
	}
}

// ListUserTasks lists tasks for the matching query or for the current user if query is nil.
func (c *Client) ListUserTasks(ctx context.Context, query *UserTaskQuery) (chan UserTaskInstance, chan error) {
	ret := make(chan UserTaskInstance, 1)
	retErr := make(chan error, 1)
	req := &model.ListUserTasksRequest{
		Owner: query.UserID,
		Group: query.GroupID,
	}
	ctx = subj.SetNS(ctx, c.ns)
	if err := api2.CallReturnStream(ctx, c.txCon, messages.APIListUserTasks, c.ExpectedCompatibleServerVersion, c.SendMiddleware, req, &model.ListUserTasksResponse{}, func(ut *model.ListUserTasksResponse) error {
		spec, err := c.getUserTaskSpec(ctx, ut.SpecUid)
		if err != nil {
			return fmt.Errorf("getUserTaskSpec: %w", err)
		}
		v := model.NewVars()
		if err := v.Decode(ctx, ut.State); err != nil {
			return fmt.Errorf("decode variables: %w", err)
		}
		ret <- &userTaskInstance{id: ut.Id, spec: spec, lockedBy: ut.LockedBy, state: v}
		return nil
	}); err != nil {
		retErr <- c.clientErr(ctx, err)
	}
	return ret, retErr
}

func (c *Client) getUserTaskSpec(ctx context.Context, uid string) (*model.TaskSpec, error) {
	if spec, ok := c.userTaskSpec[uid]; ok {
		c.userTaskSpec[uid] = spec
		return spec, nil
	}
	spec, err := c.GetTaskSpecByUID(ctx, uid)
	if err != nil {
		return nil, fmt.Errorf("GetTaskSpecByUID: %w", err)
	}
	return spec, nil
}

// OpenUserTask locks a task to the current user, and returns its specification and state
func (c *Client) OpenUserTask(ctx context.Context, taskID string, opts ...OpenUserTaskOpts) (spec *model.TaskSpec, state model.Vars, err error) {
	ctx = subj.SetNS(ctx, c.ns)
	req := &model.OpenUserTaskRequest{
		Id: taskID,
	}
	res := &model.OpenUserTaskResponse{}
	if err := api2.Call(ctx, c.con, messages.APIOpenUserTask, c.ExpectedCompatibleServerVersion, c.SendMiddleware, req, res); err != nil {
		return nil, nil, fmt.Errorf("open user task: %w", err)
	}
	job, err := c.GetJob(ctx, taskID)
	if err != nil {
		return nil, nil, fmt.Errorf("get job: %w", err)
	}
	spec, err = c.getUserTaskSpec(ctx, job.ExecuteVersion)
	if err != nil {
		return nil, nil, fmt.Errorf("getUserTaskSpec: %w", err)
	}
	v := model.NewVars()
	if err := v.Decode(ctx, res.State); err != nil {
		return nil, nil, fmt.Errorf("decode variables: %w", err)
	}
	return spec, v, nil
}

// SaveUserTaskState saves variable state into the current task.  Additive unless overwrite is set.
func (c *Client) SaveUserTaskState(ctx context.Context, taskID string, state model.Vars, overwrite bool) error {
	ctx = subj.SetNS(ctx, c.ns)
	b, err := state.Encode(ctx)
	if err != nil {
		return fmt.Errorf("encode vars: %w", err)
	}
	req := &model.SaveUserTaskRequest{
		Id:        taskID,
		Vars:      b,
		Overwrite: overwrite,
	}
	if err := api2.Call(ctx, c.con, messages.ApiSaveUserTask, c.ExpectedCompatibleServerVersion, c.SendMiddleware, req, &model.SaveUserTaskResponse{}); err != nil {
		return fmt.Errorf("call saveUserTask: %w", err)
	}
	return nil
}

// AbandonUserTask abandons a user task, releasing it back to the available task list.
func (c *Client) AbandonUserTask(ctx context.Context, taskID string, keepData bool) error {
	return nil
}

// CompleteUserTask completes a user task, with the current stored state.
func (c *Client) CompleteUserTask(ctx context.Context, taskID string) error {
	ctx = subj.SetNS(ctx, c.ns)
	req := &model.CompleteUserTaskRequest{
		Id: taskID,
	}
	res := &model.CompleteUserTaskResponse{}
	if err := api2.Call(ctx, c.con, messages.APICompleteUserTask, c.ExpectedCompatibleServerVersion, c.SendMiddleware, req, res); err != nil {
		return fmt.Errorf("call saveUserTask: %w", err)
	}
	return nil
}
