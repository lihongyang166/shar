package main

import (
	"fmt"
	"github.com/goccy/go-yaml"
	"github.com/nats-io/nats.go"
	"gitlab.com/shar-workflow/shar/common/setup"
)

func main() {
	/*
		cfg := &setup.NatsConfig{
			Streams: []setup.NatsStream{
				{
					Config: nats.StreamConfig{
						Name:      "WORKFLOW",
						Subjects:  messages.AllMessages,
						Storage:   nats.MemoryStorage,
						Retention: nats.InterestPolicy,
					},
					Consumers: []setup.NatsConsumer{
						{
							Config: nats.ConsumerConfig{
								Durable:         "JobAbortConsumer",
								Description:     "Abort job message queue",
								AckPolicy:       nats.AckExplicitPolicy,
								AckWait:         30 * time.Second,
								MaxAckPending:   65535,
								FilterSubject:   subj.NS(messages.WorkFlowJobAbortAll, "*"),
								MaxRequestBatch: 1,
							},
						},
						{
							Config: nats.ConsumerConfig{
								Durable:         "GeneralAbortConsumer",
								Description:     "Abort workflow instance and activity message queue",
								AckPolicy:       nats.AckExplicitPolicy,
								AckWait:         30 * time.Second,
								MaxAckPending:   65535,
								FilterSubject:   subj.NS(messages.WorkflowGeneralAbortAll, "*"),
								MaxRequestBatch: 1,
							},
						},
						{
							Config: nats.ConsumerConfig{
								Durable:         "LaunchConsumer",
								Description:     "Sub workflow launch message queue",
								AckPolicy:       nats.AckExplicitPolicy,
								AckWait:         30 * time.Second,
								MaxAckPending:   65535,
								FilterSubject:   subj.NS(messages.WorkflowJobLaunchExecute, "*"),
								MaxRequestBatch: 1,
							},
						},
						{
							Config: nats.ConsumerConfig{
								Durable:         "Message",
								Description:     "Intra-workflow message queue",
								AckPolicy:       nats.AckExplicitPolicy,
								AckWait:         30 * time.Second,
								MaxAckPending:   65535,
								FilterSubject:   subj.NS(messages.WorkflowMessage, "*"),
								MaxRequestBatch: 1,
							},
						},
						{
							Config: nats.ConsumerConfig{
								Durable:         "WorkflowConsumer",
								Description:     "Workflow processing queue",
								AckPolicy:       nats.AckExplicitPolicy,
								AckWait:         30 * time.Second,
								MaxAckPending:   65535,
								FilterSubject:   subj.NS(messages.WorkflowExecutionAll, "*"),
								MaxRequestBatch: 1,
							},
						},
						{
							Config: nats.ConsumerConfig{
								Durable:         "ProcessCompleteConsumer",
								Description:     "Process complete processing queue",
								AckPolicy:       nats.AckExplicitPolicy,
								AckWait:         30 * time.Second,
								MaxAckPending:   65535,
								FilterSubject:   subj.NS(messages.WorkflowProcessComplete, "*"),
								MaxRequestBatch: 1,
							},
						},
						{
							Config: nats.ConsumerConfig{
								Durable:         "JobCompleteConsumer",
								Description:     "Job complete processing queue",
								AckPolicy:       nats.AckExplicitPolicy,
								AckWait:         30 * time.Second,
								MaxAckPending:   65535,
								FilterSubject:   subj.NS(messages.WorkFlowJobCompleteAll, "*"),
								MaxRequestBatch: 1,
							},
						},
						{
							Config: nats.ConsumerConfig{
								Durable:         "ActivityConsumer",
								Description:     "Activity complete processing queue",
								AckPolicy:       nats.AckExplicitPolicy,
								AckWait:         30 * time.Second,
								MaxAckPending:   65535,
								FilterSubject:   subj.NS(messages.WorkflowActivityAll, "*"),
								MaxRequestBatch: 1,
							},
						},
						{
							Config: nats.ConsumerConfig{
								Durable:         "Traversal",
								Description:     "Traversal processing queue",
								AckPolicy:       nats.AckExplicitPolicy,
								AckWait:         30 * time.Second,
								MaxAckPending:   65535,
								FilterSubject:   subj.NS(messages.WorkflowTraversalExecute, "*"),
								MaxRequestBatch: 1,
							},
						},
						{
							Config: nats.ConsumerConfig{
								Durable:         "Tracking",
								Description:     "Tracking queue for sequential processing",
								AckPolicy:       nats.AckExplicitPolicy,
								AckWait:         30 * time.Second,
								FilterSubject:   "WORKFLOW.>",
								MaxAckPending:   1,
								MaxRequestBatch: 1,
							},
						},
						{
							Config: nats.ConsumerConfig{
								Durable:         "GatewayExecuteConsumer",
								Description:     "Tracking queue for gateway execution",
								AckPolicy:       nats.AckExplicitPolicy,
								AckWait:         30 * time.Second,
								FilterSubject:   subj.NS(messages.WorkflowJobGatewayTaskExecute, "*"),
								MaxAckPending:   65535,
								MaxRequestBatch: 1,
							},
						},
						{
							Config: nats.ConsumerConfig{
								Durable:         "GatewayActivateConsumer",
								Description:     "Tracking queue for gateway activation",
								AckPolicy:       nats.AckExplicitPolicy,
								AckWait:         30 * time.Second,
								FilterSubject:   subj.NS(messages.WorkflowJobGatewayTaskActivate, "*"),
								MaxAckPending:   1,
								MaxRequestBatch: 1,
							},
						},
						{
							Config: nats.ConsumerConfig{
								Durable:         "GatewayReEnterConsumer",
								Description:     "Tracking queue for gateway activation",
								AckPolicy:       nats.AckExplicitPolicy,
								AckWait:         30 * time.Second,
								FilterSubject:   subj.NS(messages.WorkflowJobGatewayTaskReEnter, "*"),
								MaxAckPending:   65535,
								MaxRequestBatch: 1,
							},
						},
						{
							Config: nats.ConsumerConfig{
								Durable:         "AwaitMessageConsumer",
								Description:     "Tracking queue for gateway activation",
								AckPolicy:       nats.AckExplicitPolicy,
								AckWait:         30 * time.Second,
								FilterSubject:   subj.NS(messages.WorkflowJobAwaitMessageExecute, "*"),
								MaxAckPending:   65535,
								MaxRequestBatch: 1,
							},
						},

						{
							Config: nats.ConsumerConfig{
								Durable:         "MessageKickConsumer",
								Description:     "Message processing consumer timer",
								AckPolicy:       nats.AckExplicitPolicy,
								AckWait:         120 * time.Second,
								FilterSubject:   subj.NS(messages.WorkflowMessageKick, "*"),
								MaxAckPending:   1,
								MaxRequestBatch: 1,
							},
						},
						{
							Config: nats.ConsumerConfig{
								Durable:         "TelemetryTimerConsumer",
								Description:     "Server telemetry timer",
								AckPolicy:       nats.AckExplicitPolicy,
								AckWait:         1 * time.Second,
								FilterSubject:   subj.NS(messages.WorkflowTelemetryTimer, "*"),
								MaxAckPending:   1,
								MaxRequestBatch: 1,
							},
						},
						{
							Config: nats.ConsumerConfig{
								Durable:       "UndeliveredMsgConsumer",
								Description:   "Undeliverable workflow message queue.  Messages should be looked up in the WORKFLOW-MIRROR stream to avoid disappointment.",
								AckPolicy:     nats.AckExplicitPolicy,
								AckWait:       1 * time.Second,
								FilterSubject: "$JS.EVENT.ADVISORY.CONSUMER.MAX_DELIVERIES.WORKFLOW.>",
							},
						},
					},
				},
				{
					Config: nats.StreamConfig{
						Name:      "WORKFLOW-MIRROR",
						Storage:   nats.MemoryStorage,
						Retention: nats.LimitsPolicy,
						MaxAge:    36 * time.Hour,
						Sources: []*nats.StreamSource{
							{
								Name:          "WORKFLOW",
								FilterSubject: "WORKFLOW.*.State.>",
							},
						},
					},
				},
			},
		}
	*/
	cfg := &setup.NatsConfig{
		Streams: []setup.NatsStream{
			{
				Config: nats.StreamConfig{
					Name:      "WORKFLOW_TELEMETRY",
					Subjects:  []string{"WORKFLOW_TELEMETRY.>"},
					Retention: nats.InterestPolicy,
					Sources: []*nats.StreamSource{
						{Name: "WORKFLOW", FilterSubject: "WORKFLOW.*.State.>"},
					},
				},
			},
		},
	}
	b, _ := yaml.Marshal(cfg)
	fmt.Println(string(b))
}
