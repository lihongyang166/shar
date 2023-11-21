package setup

import (
	"context"
	"errors"
	"fmt"
	"github.com/hashicorp/go-version"
	"github.com/nats-io/nats.go"
	"gitlab.com/shar-workflow/shar/common"
	"gitlab.com/shar-workflow/shar/common/setup/upgrader"
	"gitlab.com/shar-workflow/shar/common/subj"
	sharVersion "gitlab.com/shar-workflow/shar/common/version"
	"gitlab.com/shar-workflow/shar/server/messages"
	"regexp"
	"time"
)

var consumerConfig []*nats.ConsumerConfig

// ConsumerDurableNames is a list of all consumers used by the engine
var ConsumerDurableNames map[string]struct{}

func init() {
	consumerConfig = []*nats.ConsumerConfig{
		{
			Durable:         "JobAbortConsumer",
			Description:     "Abort job message queue",
			AckPolicy:       nats.AckExplicitPolicy,
			AckWait:         30 * time.Second,
			MaxAckPending:   65535,
			FilterSubject:   subj.NS(messages.WorkFlowJobAbortAll, "*"),
			MaxRequestBatch: 1,
		},
		{
			Durable:         "GeneralAbortConsumer",
			Description:     "Abort workflow instance and activity message queue",
			AckPolicy:       nats.AckExplicitPolicy,
			AckWait:         30 * time.Second,
			MaxAckPending:   65535,
			FilterSubject:   subj.NS(messages.WorkflowGeneralAbortAll, "*"),
			MaxRequestBatch: 1,
		},
		{
			Durable:         "LaunchConsumer",
			Description:     "Sub workflow launch message queue",
			AckPolicy:       nats.AckExplicitPolicy,
			AckWait:         30 * time.Second,
			MaxAckPending:   65535,
			FilterSubject:   subj.NS(messages.WorkflowJobLaunchExecute, "*"),
			MaxRequestBatch: 1,
		},
		{
			Durable:         "Message",
			Description:     "Intra-workflow message queue",
			AckPolicy:       nats.AckExplicitPolicy,
			AckWait:         30 * time.Second,
			MaxAckPending:   65535,
			FilterSubject:   subj.NS(messages.WorkflowMessage, "*"),
			MaxRequestBatch: 1,
		},
		{
			Durable:         "WorkflowConsumer",
			Description:     "Workflow processing queue",
			AckPolicy:       nats.AckExplicitPolicy,
			AckWait:         30 * time.Second,
			MaxAckPending:   65535,
			FilterSubject:   subj.NS(messages.ExecutionAll, "*"),
			MaxRequestBatch: 1,
		},
		{
			Durable:         "ProcessCompleteConsumer",
			Description:     "Process complete processing queue",
			AckPolicy:       nats.AckExplicitPolicy,
			AckWait:         30 * time.Second,
			MaxAckPending:   65535,
			FilterSubject:   subj.NS(messages.WorkflowProcessComplete, "*"),
			MaxRequestBatch: 1,
		},
		{
			Durable:         "JobCompleteConsumer",
			Description:     "Job complete processing queue",
			AckPolicy:       nats.AckExplicitPolicy,
			AckWait:         30 * time.Second,
			MaxAckPending:   65535,
			FilterSubject:   subj.NS(messages.WorkFlowJobCompleteAll, "*"),
			MaxRequestBatch: 1,
		},
		{
			Durable:         "ActivityConsumer",
			Description:     "Activity complete processing queue",
			AckPolicy:       nats.AckExplicitPolicy,
			AckWait:         30 * time.Second,
			MaxAckPending:   65535,
			FilterSubject:   subj.NS(messages.WorkflowActivityAll, "*"),
			MaxRequestBatch: 1,
		},
		{
			Durable:         "Traversal",
			Description:     "Traversal processing queue",
			AckPolicy:       nats.AckExplicitPolicy,
			AckWait:         30 * time.Second,
			MaxAckPending:   65535,
			FilterSubject:   subj.NS(messages.WorkflowTraversalExecute, "*"),
			MaxRequestBatch: 1,
		},
		{
			Durable:         "Tracking",
			Description:     "Tracking queue for sequential processing",
			AckPolicy:       nats.AckExplicitPolicy,
			AckWait:         30 * time.Second,
			FilterSubject:   "WORKFLOW.>",
			MaxAckPending:   1,
			MaxRequestBatch: 1,
		},
		{
			Durable:         "GatewayExecuteConsumer",
			Description:     "Tracking queue for gateway execution",
			AckPolicy:       nats.AckExplicitPolicy,
			AckWait:         30 * time.Second,
			FilterSubject:   subj.NS(messages.WorkflowJobGatewayTaskExecute, "*"),
			MaxAckPending:   65535,
			MaxRequestBatch: 1,
		},
		{
			Durable:         "GatewayActivateConsumer",
			Description:     "Tracking queue for gateway activation",
			AckPolicy:       nats.AckExplicitPolicy,
			AckWait:         30 * time.Second,
			FilterSubject:   subj.NS(messages.WorkflowJobGatewayTaskActivate, "*"),
			MaxAckPending:   1,
			MaxRequestBatch: 1,
		},
		{
			Durable:         "GatewayReEnterConsumer",
			Description:     "Tracking queue for gateway activation",
			AckPolicy:       nats.AckExplicitPolicy,
			AckWait:         30 * time.Second,
			FilterSubject:   subj.NS(messages.WorkflowJobGatewayTaskReEnter, "*"),
			MaxAckPending:   65535,
			MaxRequestBatch: 1,
		},
		{
			Durable:         "AwaitMessageConsumer",
			Description:     "Tracking queue for gateway activation",
			AckPolicy:       nats.AckExplicitPolicy,
			AckWait:         30 * time.Second,
			FilterSubject:   subj.NS(messages.WorkflowJobAwaitMessageExecute, "*"),
			MaxAckPending:   65535,
			MaxRequestBatch: 1,
		},
		{
			Durable:         "MessageKickConsumer",
			Description:     "Message processing consumer timer",
			AckPolicy:       nats.AckExplicitPolicy,
			AckWait:         120 * time.Second,
			FilterSubject:   subj.NS(messages.WorkflowMessageKick, "*"),
			MaxAckPending:   1,
			MaxRequestBatch: 1,
		},
		{
			Durable:         "TelemetryTimerConsumer",
			Description:     "Server telemetry timer",
			AckPolicy:       nats.AckExplicitPolicy,
			AckWait:         1 * time.Second,
			FilterSubject:   subj.NS(messages.WorkflowTelemetryTimer, "*"),
			MaxAckPending:   1,
			MaxRequestBatch: 1,
		},
	}
	ConsumerDurableNames = make(map[string]struct{}, len(consumerConfig))
	for _, v := range consumerConfig {
		ConsumerDurableNames[v.Durable] = struct{}{}
	}
}

// EnsureWorkflowStream ensures that the workflow stream exists
func EnsureWorkflowStream(ctx context.Context, nc common.NatsConn, js nats.JetStreamContext, storageType nats.StorageType, update bool) error {
	mirrorAge := 36 * time.Hour
	if err := EnsureStream(ctx, nc, js, nats.StreamConfig{
		Name:      "WORKFLOW",
		Subjects:  messages.AllMessages,
		Storage:   storageType,
		Retention: nats.InterestPolicy,
	}); err != nil {
		return fmt.Errorf("ensure workflow stream: %w", err)
	}

	if err := EnsureStream(ctx, nc, js, nats.StreamConfig{
		Name:      "WORKFLOW-MIRROR",
		Storage:   storageType,
		Retention: nats.LimitsPolicy,
		MaxAge:    mirrorAge,
		Sources: []*nats.StreamSource{{
			Name:          "WORKFLOW",
			FilterSubject: "WORKFLOW.*.State.>",
		}},
	}); err != nil {
		return fmt.Errorf("ensure workflow stream: %w", err)
	}

	if err := EnsureConsumer(js, "WORKFLOW", nats.ConsumerConfig{
		Durable:       "UndeliveredMsgConsumer",
		Description:   "Undeliverable workflow message queue.  Messages should be looked up in the WORKFLOW-MIRROR stream to avoid disappointment.",
		FilterSubject: "$JS.EVENT.ADVISORY.CONSUMER.MAX_DELIVERIES.WORKFLOW.>",
		MemoryStorage: storageType == nats.MemoryStorage,
	}, update); err != nil {
		return fmt.Errorf("ensure consumer during ensure workflow stream: %w", err)
	}

	for _, ccfg := range consumerConfig {
		if err := EnsureConsumer(js, "WORKFLOW", *ccfg, update); err != nil {
			return fmt.Errorf("ensure consumer during ensure workflow stream: %w", err)
		}
	}
	return nil
}

// EnsureConsumer creates a new consumer appending the current semantic version number to the description.  If the consumer exists and has a previous version, it upgrader it.
func EnsureConsumer(js nats.JetStreamContext, streamName string, consumerConfig nats.ConsumerConfig, update bool) error {
	if ci, err := js.ConsumerInfo(streamName, consumerConfig.Durable); errors.Is(err, nats.ErrConsumerNotFound) {
		consumerConfig.Description += " " + sharVersion.Version
		if _, err := js.AddConsumer(streamName, &consumerConfig); err != nil {
			return fmt.Errorf("cannot ensure consumer '%s' with subject '%s' : %w", consumerConfig.Name, consumerConfig.FilterSubject, err)
		}
	} else if err != nil {
		return fmt.Errorf("ensure consumer: %w", err)
	} else {
		if !update {
			return nil
		}
		if ok := requiresUpgrade(ci.Config.Description, sharVersion.Version); ok {
			consumerConfig.Description += " " + sharVersion.Version
			_, err := js.UpdateConsumer(streamName, &consumerConfig)
			if err != nil {
				return fmt.Errorf("ensure stream couldn't update the consumer configuration for %s: %w", consumerConfig.Name, err)
			}
		}
	}
	return nil
}

// EnsureStream creates a new stream appending the current semantic version number to the description.  If the stream exists and has a previous version, it upgrader it.
func EnsureStream(ctx context.Context, nc common.NatsConn, js nats.JetStreamContext, streamConfig nats.StreamConfig) error {
	if si, err := js.StreamInfo(streamConfig.Name); errors.Is(err, nats.ErrStreamNotFound) {
		streamConfig.Description += " " + sharVersion.Version
		if _, err := js.AddStream(&streamConfig); err != nil {
			return fmt.Errorf("add stream: %w", err)
		}
	} else if err != nil {
		return fmt.Errorf("ensure stream: %w", err)
	} else {
		if ok := requiresUpgrade(si.Config.Description, sharVersion.Version); ok {
			existingVer := upgradeExpr.FindString(si.Config.Description)
			if existingVer == "" {
				existingVer = sharVersion.Version
			}
			if err := upgrader.Patch(ctx, existingVer, nc, js); err != nil {
				return fmt.Errorf("ensure stream: %w", err)
			}
			streamConfig.Description += " " + sharVersion.Version
			_, err := js.UpdateStream(&streamConfig)
			if err != nil {
				return fmt.Errorf("ensure stream updating stream configuration: %w", err)
			}
		}
	}
	return nil
}

// upgradeExpr is the version check regex
var upgradeExpr = regexp.MustCompilePOSIX(`([0-9])*\.([0-9])*\.([0-9])*$`)

// requiresUpgrade reads the description on an existing SHAR JetStream object.  It compares this with the running version and returs true if an upgrade is needed.
func requiresUpgrade(description string, newVersion string) bool {

	if v := upgradeExpr.FindString(description); len(v) == 0 {
		return true
	} else {
		v1, err := version.NewVersion(v)
		if err != nil {
			return true
		}
		v2, err := version.NewVersion(newVersion)
		if err != nil {
			return true
		}
		return v2.GreaterThanOrEqual(v1)
	}
}
