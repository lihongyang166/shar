package storage

import (
	"context"
	"errors"
	"fmt"
	"github.com/nats-io/nats.go"
	"github.com/segmentio/ksuid"
	"gitlab.com/shar-workflow/shar/common"
	"gitlab.com/shar-workflow/shar/common/task"
	"gitlab.com/shar-workflow/shar/model"
)

func (s *Nats) GetTaskSpecUID(ctx context.Context, name string) (string, error) {
	tskVer := &model.TaskSpecVersions{}
	if err := common.LoadObj(ctx, s.wfTaskSpecVer, name, tskVer); errors.Is(err, nats.ErrKeyNotFound) {
		// Try the legacy store
		if b, err2 := common.Load(ctx, s.wfClientTask, name); errors.Is(err2, nats.ErrKeyNotFound) {
			return "", fmt.Errorf("get task spec id: %w", err2)
		} else if err2 != nil {
			return "", fmt.Errorf("get legacy task spec id: %w", err2)
		} else {
			return string(b), nil
		}
	} else if err != nil {
		return "", fmt.Errorf("opening task spec versions: %w", err)
	}

	return tskVer.Id[len(tskVer.Id)-1], nil
}

func (s *Nats) PutTaskSpec(ctx context.Context, spec *model.TaskSpec) (string, error) {
	// Legacy task registration
	if spec.Version == task.LegacyTask {
		return s.putLegacyTaskSpec(ctx, spec)
	}

	uid, err := task.CreateUID(spec)
	if err != nil {
		return "", fmt.Errorf("put task spec: hask task: %w", err)
	}
	spec.Metadata.Uid = uid
	if err := common.SaveObj(ctx, s.wfTaskSpec, spec.Metadata.Type, spec); err != nil {
		return "", fmt.Errorf("saving task spec: %w", err)
	}
	vers := &model.TaskSpecVersions{}
	if err := common.UpdateObj(ctx, s.wfTaskSpecVer, spec.Metadata.Type, vers, func(v *model.TaskSpecVersions) (*model.TaskSpecVersions, error) {
		v.Id = append(v.Id, uid)
		return v, nil
	}); err != nil {
		return "", fmt.Errorf("saving task spec version: %w", err)
	}
	return uid, nil
}

func (s *Nats) putLegacyTaskSpec(ctx context.Context, spec *model.TaskSpec) (string, error) {
	id, err := ksuid.NewRandom()
	if err != nil {
		return "", fmt.Errorf("put legacy task ksuid generation: %w", err)
	}
	if err := common.Save(ctx, s.wfClientTask, spec.Metadata.Type, []byte(id.String())); err != nil {
		return "", fmt.Errorf("saving legacy task routing: %w", err)
	}
	return id.String(), nil
}
