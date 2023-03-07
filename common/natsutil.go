package common

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"github.com/nats-io/nats.go"
	"gitlab.com/shar-workflow/shar/common/header"
	"gitlab.com/shar-workflow/shar/common/logx"
	"gitlab.com/shar-workflow/shar/common/workflow"
	errors2 "gitlab.com/shar-workflow/shar/server/errors"
	"golang.org/x/exp/slog"
	"google.golang.org/protobuf/proto"
	"math/big"
	"reflect"
	"strconv"
	"strings"
	"time"
)

const EphemeralConsumer = "EPHEMERAL_CONSUMER_CONFIG"

// NatsConn is the trimmad down NATS Connection interface that only emcompasses the methods used by SHAR
type NatsConn interface {
	JetStream(opts ...nats.JSOpt) (nats.JetStreamContext, error)
	QueueSubscribe(subj string, queue string, cb nats.MsgHandler) (*nats.Subscription, error)
}

func updateKV(wf nats.KeyValue, k string, msg proto.Message, updateFn func(v []byte, msg proto.Message) ([]byte, error)) error {
	for {
		entry, err := wf.Get(k)
		if err != nil {
			return fmt.Errorf("failed to get value to update: %w", err)
		}
		rev := entry.Revision()
		uv, err := updateFn(entry.Value(), msg)
		if err != nil {
			return fmt.Errorf("failed during update function: %w", err)
		}
		_, err = wf.Update(k, uv, rev)
		// TODO: Horrible workaround for the fact that this is not a typed error
		if err != nil {
			maxJitter := &big.Int{}
			maxJitter.SetInt64(5000)
			if strings.Contains(err.Error(), "wrong last sequence") {
				dur, err := rand.Int(rand.Reader, maxJitter) // Jitter
				if err != nil {
					panic("could not read random")
				}
				time.Sleep(time.Duration(dur.Int64()))
				continue
			}
			return fmt.Errorf("failed to update kv: %w", err)
		}
		break
	}
	return nil
}

// Save saves a value to a key value store
func Save(ctx context.Context, wf nats.KeyValue, k string, v []byte) error {
	log := slog.FromContext(ctx)
	if log.Enabled(errors2.VerboseLevel) {
		log.Log(errors2.VerboseLevel, "Set KV", slog.String("bucket", wf.Bucket()), slog.String("key", k), slog.String("val", string(v)))
	}
	if _, err := wf.Put(k, v); err != nil {
		return fmt.Errorf("failed to save kv: %w", err)
	}
	return nil
}

// Load loads a value from a key value store
func Load(ctx context.Context, wf nats.KeyValue, k string) ([]byte, error) {
	log := slog.FromContext(ctx)
	if log.Enabled(errors2.VerboseLevel) {
		log.Log(errors2.VerboseLevel, "Get KV", slog.Any("bucket", wf.Bucket()), slog.String("key", k))
	}
	b, err := wf.Get(k)
	if err == nil {
		return b.Value(), nil
	}
	return nil, fmt.Errorf("failed to load value from KV: %w", err)
}

// SaveObj save an protobuf message to a key value store
func SaveObj(ctx context.Context, wf nats.KeyValue, k string, v proto.Message) error {
	log := slog.FromContext(ctx)
	if log.Enabled(errors2.TraceLevel) {
		log.Log(errors2.TraceLevel, "save KV object", slog.String("bucket", wf.Bucket()), slog.String("key", k), slog.Any("val", v))
	}
	b, err := proto.Marshal(v)
	if err == nil {
		return Save(ctx, wf, k, b)
	}
	return fmt.Errorf("failed to save object into KV: %w", err)
}

// LoadObj loads a protobuf message from a key value store
func LoadObj(ctx context.Context, wf nats.KeyValue, k string, v proto.Message) error {
	log := slog.FromContext(ctx)
	if log.Enabled(errors2.TraceLevel) {
		log.Log(errors2.TraceLevel, "load KV object", slog.String("bucket", wf.Bucket()), slog.String("key", k), slog.Any("val", v))
	}
	kv, err := Load(ctx, wf, k)
	if err != nil {
		return fmt.Errorf("failed to load object from KV %s(%s): %w", wf.Bucket(), k, err)
	}
	if err := proto.Unmarshal(kv, v); err != nil {
		return fmt.Errorf("failed to unmarshal in LoadObj: %w", err)
	}
	return nil
}

// UpdateObj saves an protobuf message to a key value store after using updateFN to update the message.
func UpdateObj[T proto.Message](ctx context.Context, wf nats.KeyValue, k string, msg T, updateFn func(v T) (T, error)) error {
	log := slog.FromContext(ctx)
	if log.Enabled(errors2.TraceLevel) {
		log.Log(errors2.TraceLevel, "update KV object", slog.String("bucket", wf.Bucket()), slog.String("key", k), slog.Any("fn", reflect.TypeOf(updateFn)))
	}
	if oldk, err := wf.Get(k); errors.Is(err, nats.ErrKeyNotFound) || (err == nil && oldk.Value() == nil) {
		if err := SaveObj(ctx, wf, k, msg); err != nil {
			return fmt.Errorf("failed to save during update object: %w", err)
		}
	}
	return updateKV(wf, k, msg, func(bv []byte, msg proto.Message) ([]byte, error) {
		if err := proto.Unmarshal(bv, msg); err != nil {
			return nil, fmt.Errorf("failed to unmarshal proto for KV update: %w", err)
		}
		uv, err := updateFn(msg.(T))
		if err != nil {
			return nil, fmt.Errorf("failed during update function: %w", err)
		}
		b, err := proto.Marshal(uv)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal updated proto: %w", err)
		}
		return b, nil
	})
}

// UpdateObjIsNew saves an protobuf message to a key value store after using updateFN to update the message, and returns true if this is a new value.
func UpdateObjIsNew[T proto.Message](ctx context.Context, wf nats.KeyValue, k string, msg T, updateFn func(v T) (T, error)) (bool, error) {
	log := slog.FromContext(ctx)
	if log.Enabled(errors2.TraceLevel) {
		log.Log(errors2.TraceLevel, "update KV object", slog.String("bucket", wf.Bucket()), slog.String("key", k), slog.Any("fn", reflect.TypeOf(updateFn)))
	}
	isNew := false
	if oldk, err := wf.Get(k); errors.Is(err, nats.ErrKeyNotFound) || (err == nil && oldk.Value() == nil) {
		if err := SaveObj(ctx, wf, k, msg); err != nil {
			return false, fmt.Errorf("failed to save during update object: %w", err)
		}
		isNew = true
	}

	if err := updateKV(wf, k, msg, func(bv []byte, msg proto.Message) ([]byte, error) {
		if err := proto.Unmarshal(bv, msg); err != nil {
			return nil, fmt.Errorf("failed to unmarshal proto for KV update: %w", err)
		}
		uv, err := updateFn(msg.(T))
		if err != nil {
			return nil, fmt.Errorf("failed during update function: %w", err)
		}
		b, err := proto.Marshal(uv)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal updated proto: %w", err)
		}
		return b, nil
	}); err != nil {
		return false, fmt.Errorf("update obj is new failed: %w", err)
	}
	return isNew, nil
}

// Delete deletes an item from a key value store.
func Delete(kv nats.KeyValue, key string) error {
	if err := kv.Delete(key); err != nil {
		return fmt.Errorf("failed to delete key: %w", err)
	}
	return nil
}

// EnsureBuckets ensures that a list of key value stores exist
func EnsureBuckets(js nats.JetStreamContext, storageType nats.StorageType, names []string) error {
	for _, i := range names {
		if _, err := js.KeyValue(i); errors.Is(err, nats.ErrBucketNotFound) {
			if _, err := js.CreateKeyValue(&nats.KeyValueConfig{Bucket: i, Storage: storageType}); err != nil {
				return fmt.Errorf("failed to ensure buckets: %w", err)
			}
		} else if err != nil {
			return fmt.Errorf("failed to obtain bucket: %w", err)
		}
	}
	return nil
}

// Process processes messages from a nats consumer and executes a function against each one.
func Process(ctx context.Context, js nats.JetStreamContext, traceName string, closer chan struct{}, subject string, durable string, concurrency int, fn func(ctx context.Context, log *slog.Logger, msg *nats.Msg) (bool, error)) error {
	log := slog.FromContext(ctx)
	if !strings.HasPrefix(durable, "ServiceTask_") {
		conInfo, err := js.ConsumerInfo("WORKFLOW", durable)
		if conInfo.Config.Durable == "" || err != nil {
			return fmt.Errorf("durable consumer '%s' is not explicity configured", durable)
		}
		if conInfo.Config.Durable == EphemeralConsumer {
			conInfo.Config.Durable = ""
		}
	}
	sub, err := js.PullSubscribe(subject, durable)
	if err != nil {
		log.Error("process pull subscribe error", err, "subject", subject, "durable", durable)
		return fmt.Errorf("process pull subscribe error subject:%s durable:%s: %w", subject, durable, err)
	}
	for i := 0; i < concurrency; i++ {
		go func() {
			for {
				select {
				case <-closer:
					return
				default:
				}
				reqCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
				msg, err := sub.Fetch(1, nats.Context(reqCtx))
				if err != nil {
					if errors.Is(err, context.DeadlineExceeded) {
						cancel()
						continue
					}
					// Horrible, but this isn't a typed error.  This test just stops the listener printing pointless errors.
					if err.Error() == "nats: Server Shutdown" || err.Error() == "nats: connection closed" {
						cancel()
						continue
					}
					// Log Error
					log.Error("message fetch error", err)
					cancel()
					continue
				}
				m := msg[0]
				ctx, err := header.FromMsgHeaderToCtx(ctx, m.Header)
				if err != nil {
					log.Error("failed to get header values from incoming process message", &errors2.ErrWorkflowFatal{Err: err})
					if err := msg[0].Ack(); err != nil {
						log.Error("processing failed to ack", err)
					}
					cancel()
					continue
				}
				//				log.Debug("Process:"+traceName, slog.String("subject", msg[0].Subject))
				cancel()
				if embargo := m.Header.Get("embargo"); embargo != "" && embargo != "0" {
					e, err := strconv.Atoi(embargo)
					if err != nil {
						log.Error("bad embargo value", err)
						continue
					}
					offset := time.Duration(int64(e) - time.Now().UnixNano())
					if offset > 0 {
						if err != m.NakWithDelay(offset) {
							log.Warn("failed to nak with delay")
						}
						continue
					}
				}
				executeCtx, executeLog := logx.NatsMessageLoggingEntrypoint(context.Background(), "server", m.Header)
				executeCtx = header.Copy(ctx, executeCtx)
				ack, err := fn(executeCtx, executeLog, msg[0])
				if err != nil {
					if errors2.IsWorkflowFatal(err) {
						executeLog.Error("workflow fatal error occured processing function", err)
						ack = true
					} else {
						wfe := &workflow.Error{}
						if !errors.As(err, wfe) {
							executeLog.Error("processing error", err, "name", traceName)
						}
					}
				}
				if ack {
					if err := msg[0].Ack(); err != nil {
						log.Error("processing failed to ack", err)
					}
				} else {
					if err := msg[0].Nak(); err != nil {
						log.Error("processing failed to nak", err)
					}
				}
			}
		}()
	}
	return nil
}
