package services

import (
	"context"
	"errors"
	"fmt"
	"github.com/crystal-construct/shar/internal/messages"
	"github.com/crystal-construct/shar/model"
	"github.com/crystal-construct/shar/telemetry/ctxutil"
	"github.com/nats-io/nats.go"
	"github.com/uptrace/opentelemetry-go-extra/otelzap"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
	"google.golang.org/protobuf/proto"
	"time"
)

type NatsQueue struct {
	js                        nats.JetStreamContext
	con                       *nats.Conn
	eventProcessor            EventProcessorFunc
	eventJobCompleteProcessor CompleteJobProcessorFunc
	log                       *otelzap.Logger
	storageType               nats.StorageType
	concurrency               int
	tracer                    trace.Tracer
	closing                   chan struct{}
}

func NewNatsQueue(log *otelzap.Logger, conn *nats.Conn, storageType nats.StorageType, tracer trace.Tracer, concurrency int) (*NatsQueue, error) {
	if concurrency < 1 || concurrency > 200 {
		return nil, errors.New("invalid concurrency set")
	}
	js, err := conn.JetStream()
	if err != nil {
		return nil, err
	}
	return &NatsQueue{
		concurrency: concurrency,
		storageType: storageType,
		con:         conn,
		js:          js,
		log:         log,
		tracer:      tracer,
		closing:     make(chan struct{}),
	}, nil
}

func (q *NatsQueue) Traverse(ctx context.Context, workflowInstanceId, elementId string, vars []byte) error {
	b, err := proto.Marshal(&model.Traversal{
		ElementId:          elementId,
		WorkflowInstanceId: workflowInstanceId,
		Vars:               vars,
	})
	if err != nil {
		return fmt.Errorf("failed to marshal traversal: %w", err)
	}
	msg := nats.NewMsg(messages.WorkflowTraversal)
	msg.Data = b
	ctxutil.LoadNATSHeaderFromContext(ctx, msg)
	if _, err = q.js.PublishMsg(msg); err != nil {
		return err
	}
	return nil
}

func (q *NatsQueue) StartProcessing(ctx context.Context) error {
	scfg := &nats.StreamConfig{
		Name: "WORKFLOW",
		Subjects: []string{
			messages.WorkflowTraversal,
			messages.WorkflowJobExecuteAll,
			messages.WorkFlowJobCompleteAll,
			messages.WorkflowInstanceAll,
			messages.WorkflowActivityAll,
		},
		Storage: q.storageType,
	}

	ccfg := &nats.ConsumerConfig{
		Durable:       "Traversal",
		AckPolicy:     nats.AckExplicitPolicy,
		FilterSubject: messages.WorkflowTraversal,
	}

	if _, err := q.js.StreamInfo(scfg.Name); err == nats.ErrStreamNotFound {
		if _, err := q.js.AddStream(scfg); err != nil {
			panic(err)
		}
	} else if err != nil {
		panic(err)
	}

	if _, err := q.js.ConsumerInfo(scfg.Name, ccfg.Durable); err == nats.ErrConsumerNotFound {
		if _, err := q.js.AddConsumer("WORKFLOW", ccfg); err != nil {
			panic(err)
		}
	} else if err != nil {
		panic(err)
	}

	go func() {
		q.processTraversals(ctx)
	}()
	go q.processCompletedJobs(ctx)
	return nil
}
func (q *NatsQueue) SetEventProcessor(processor EventProcessorFunc) {
	q.eventProcessor = processor
}
func (q *NatsQueue) SetCompleteJobProcessor(processor CompleteJobProcessorFunc) {
	q.eventJobCompleteProcessor = processor
}

func (q *NatsQueue) PublishJob(ctx context.Context, stateName string, el *model.Element, job *model.Job) error {
	return q.PublishWorkflowState(ctx, stateName, job)
}

func (q *NatsQueue) PublishWorkflowState(ctx context.Context, stateName string, message proto.Message) error {
	msg := nats.NewMsg(stateName)
	if b, err := proto.Marshal(message); err != nil {
		return err
	} else {
		msg.Data = b
	}
	ctxutil.LoadNATSHeaderFromContext(ctx, msg)
	if _, err := q.js.PublishMsg(msg); err != nil {
		return err
	}
	return nil
}

func (q *NatsQueue) processTraversals(ctx context.Context) {
	q.process(ctx, messages.WorkflowTraversal, "Traversal", func(ctx context.Context, msg *nats.Msg) error {
		var traversal model.Traversal
		if err := proto.Unmarshal(msg.Data, &traversal); err != nil {
			return fmt.Errorf("could not unmarshal traversal proto: %w", err)
		}
		if q.eventProcessor != nil {
			if err := q.eventProcessor(ctx, traversal.WorkflowInstanceId, traversal.ElementId, traversal.Vars); err != nil {
				return fmt.Errorf("could not process event: %w", err)
			}
		}
		return nil
	})
	return
}

func (q *NatsQueue) processCompletedJobs(ctx context.Context) {
	q.process(ctx, messages.WorkFlowJobCompleteAll, "JobCompleteConsumer", func(ctx context.Context, msg *nats.Msg) error {
		var job model.Job
		if err := proto.Unmarshal(msg.Data, &job); err != nil {
			return err
		}
		if q.eventJobCompleteProcessor != nil {
			if err := q.eventJobCompleteProcessor(ctx, job.Id, job.Vars); err != nil {
				return err
			}
		}
		return nil
	})
}

func (q *NatsQueue) process(ctx context.Context, subject string, durable string, fn func(ctx context.Context, msg *nats.Msg) error) {
	for i := 0; i < q.concurrency; i++ {
		go func() {
			sub, err := q.js.PullSubscribe(subject, durable)
			if err != nil {
				q.log.Ctx(ctx).Error("process pull subscribe error", zap.Error(err), zap.String("subject", subject))
				return
			}
			for {
				select {
				case <-q.closing:
					return
				default:
				}
				ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
				msg, err := sub.Fetch(1, nats.Context(ctx))
				if err != nil {
					if err == context.DeadlineExceeded {
						cancel()
						continue
					}
					// Log Error
					q.log.Ctx(ctx).Error("message fetch error", zap.Error(err))
					cancel()
					return
				}
				executeCtx := ctxutil.LoadContextFromNATSHeader(ctx, msg[0])
				err = fn(executeCtx, msg[0])
				if err != nil {
					q.log.Ctx(executeCtx).Error("processing error", zap.Error(err))
				}
				if err := msg[0].Ack(); err != nil {
					q.log.Ctx(executeCtx).Error("processing failed to ack", zap.Error(err))
				}

				cancel()
			}
		}()
	}
}
