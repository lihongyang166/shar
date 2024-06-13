package upgrader

import (
	"context"
	"github.com/nats-io/nats.go/jetstream"
	"gitlab.com/shar-workflow/shar/common"
	"log/slog"
)

var ver = "1.1.503"

/*
In this upgrade the database changes are as follows:
- The task KV has changed from a string to a model.TaskInfo
*/

//goland:noinspection GoSnakeCaseUsage
func v1_1_503(ctx context.Context, nc common.NatsConn, js jetstream.JetStream) error {
	slog.Error("attempt upgrade to: " + ver)
	// *Upgrade data in service task registry
	/*
		kv, err := js.KeyValue(ctx, messages.KvClientTaskID)
		if err != nil {
			return fmt.Errorf("upgrade %s getting service task key value store: %w", ver, err)
		}
		keys, err := kv.Keys(ctx)
		if err != nil {
			return fmt.Errorf("upgrade %s getting keys: %w", ver, err)
		}
		for _, k := range keys {
			val, err := common.Load(ctx, kv, k)
			if err != nil {
				return fmt.Errorf("upgrade %s getting key %s: %w", ver, k, err)
			}
			ks, err := ksuid.Parse(string(val))
			if err != nil {
				return fmt.Errorf("parsing ksuid from task '%s' id '%s': %w", k, val, err)
			}
			if err := common.SaveObj(ctx, kv, k, &model.TaskSpec{
				Version: "1.0",
				Kind:    "Task",
				Metadata: &model.TaskMetadata{
					Uid:         ks.String(),
					Type:        k,
					Version:     "",
					Short:       "",
					Description: "",
					Labels:      nil,
				},
				Behaviour: &model.TaskBehaviour{
					EstimatedMaxDuration: uint64(5 * time.Second),
				},
				Parameters: nil,
				Events:     nil,
			}); err != nil {
				return fmt.Errorf("upgrading value %+v in %s to %s: %w", val, k, ver, err)
			}
		}

	*/
	return nil
}
