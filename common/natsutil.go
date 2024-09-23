package common

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"gitlab.com/shar-workflow/shar/model"
	"log/slog"
	"maps"
	"math/big"
	"reflect"
	"strconv"
	"strings"
	"time"

	version2 "github.com/hashicorp/go-version"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
	"gitlab.com/shar-workflow/shar/common/header"
	"gitlab.com/shar-workflow/shar/common/logx"
	"gitlab.com/shar-workflow/shar/common/middleware"
	"gitlab.com/shar-workflow/shar/common/subj"
	version3 "gitlab.com/shar-workflow/shar/common/version"
	"gitlab.com/shar-workflow/shar/common/workflow"
	errors2 "gitlab.com/shar-workflow/shar/server/errors"
	"gitlab.com/shar-workflow/shar/server/messages"
	"golang.org/x/exp/slices"
	"google.golang.org/protobuf/proto"
)

// NatsConn is the trimmed down NATS Connection interface that only encompasses the methods used by SHAR
type NatsConn interface {
	QueueSubscribe(subj string, queue string, cb nats.MsgHandler) (*nats.Subscription, error)
	Publish(subj string, bytes []byte) error
	PublishMsg(msg *nats.Msg) error
	Subscribe(subj string, cb nats.MsgHandler) (*nats.Subscription, error)
}

func updateKV(ctx context.Context, wf jetstream.KeyValue, k string, msg proto.Message, updateFn func(v []byte, msg proto.Message) ([]byte, error)) error {
	const JSErrCodeStreamWrongLastSequence = 10071
	for {
		entry, err := wf.Get(ctx, k)
		if err != nil {
			return fmt.Errorf("get value to update: %w", err)
		}
		rev := entry.Revision()
		uv, err := updateFn(entry.Value(), msg)
		if err != nil {
			return fmt.Errorf("update function: %w", err)
		}
		_, err = wf.Update(ctx, k, uv, rev)
		if err != nil {
			maxJitter := &big.Int{}
			maxJitter.SetInt64(5000)
			testErr := &jetstream.APIError{}
			if errors.As(err, &testErr) {
				if testErr.ErrorCode == JSErrCodeStreamWrongLastSequence {
					dur, err := rand.Int(rand.Reader, maxJitter) // Jitter
					if err != nil {
						panic("read random")
					}
					time.Sleep(time.Duration(dur.Int64()))
					continue
				}
			}
			return fmt.Errorf("update kv: %w", err)
		}
		break
	}
	return nil
}

// Save saves a value to a key value store
func Save(ctx context.Context, wf jetstream.KeyValue, k string, v []byte) error {
	log := logx.FromContext(ctx)
	if log.Enabled(ctx, errors2.VerboseLevel) {
		log.Log(ctx, errors2.VerboseLevel, "Set KV", slog.String("bucket", wf.Bucket()), slog.String("key", k), slog.String("val", string(v)))
	}
	if _, err := wf.Put(ctx, k, v); err != nil {
		return fmt.Errorf("save kv: %w", err)
	}
	return nil
}

// Load loads a value from a key value store
func Load(ctx context.Context, wf jetstream.KeyValue, k string) ([]byte, error) {
	log := logx.FromContext(ctx)
	if log.Enabled(ctx, errors2.VerboseLevel) {
		log.Log(ctx, errors2.VerboseLevel, "Get KV", slog.Any("bucket", wf.Bucket()), slog.String("key", k))
	}
	b, err := wf.Get(ctx, k)
	if err == nil {
		return b.Value(), nil
	}
	return nil, fmt.Errorf("load value from KV: %w", err)
}

// SaveObj save an protobuf message to a key value store
func SaveObj(ctx context.Context, wf jetstream.KeyValue, k string, v proto.Message) error {
	log := logx.FromContext(ctx)
	if log.Enabled(ctx, errors2.TraceLevel) {
		log.Log(ctx, errors2.TraceLevel, "save KV object", slog.String("bucket", wf.Bucket()), slog.String("key", k), slog.Any("val", v))
	}
	b, err := proto.Marshal(v)
	if err == nil {
		return Save(ctx, wf, k, b)
	}
	return fmt.Errorf("save object into KV: %w", err)
}

// LoadObj loads a protobuf message from a key value store
func LoadObj(ctx context.Context, wf jetstream.KeyValue, k string, v proto.Message) error {
	log := logx.FromContext(ctx)
	if log.Enabled(ctx, errors2.TraceLevel) {
		log.Log(ctx, errors2.TraceLevel, "load KV object", slog.String("bucket", wf.Bucket()), slog.String("key", k), slog.Any("val", v))
	}
	kv, err := Load(ctx, wf, k)
	if err != nil {
		return fmt.Errorf("load object from KV %s(%s): %w", wf.Bucket(), k, err)
	}
	if err := proto.Unmarshal(kv, v); err != nil {
		return fmt.Errorf("unmarshal in LoadObj: %w", err)
	}
	return nil
}

// UpdateObj saves an protobuf message to a key value store after using updateFN to update the message.
func UpdateObj[T proto.Message](ctx context.Context, wf jetstream.KeyValue, k string, msg T, updateFn func(v T) (T, error)) error {
	log := logx.FromContext(ctx)
	if log.Enabled(ctx, errors2.TraceLevel) {
		log.Log(ctx, errors2.TraceLevel, "update KV object", slog.String("bucket", wf.Bucket()), slog.String("key", k), slog.Any("fn", reflect.TypeOf(updateFn)))
	}
	if oldk, err := wf.Get(ctx, k); errors.Is(err, jetstream.ErrKeyNotFound) || (err == nil && oldk.Value() == nil) {
		if err := SaveObj(ctx, wf, k, msg); err != nil {
			return fmt.Errorf("save during update object: %w", err)
		}
	}
	return updateKV(ctx, wf, k, msg, func(bv []byte, msg proto.Message) ([]byte, error) {
		if err := proto.Unmarshal(bv, msg); err != nil {
			return nil, fmt.Errorf("unmarshal proto for KV update: %w", err)
		}
		uv, err := updateFn(msg.(T))
		if err != nil {
			return nil, fmt.Errorf("update function: %w", err)
		}
		b, err := proto.Marshal(uv)
		if err != nil {
			return nil, fmt.Errorf("marshal updated proto: %w", err)
		}
		return b, nil
	})
}

// UpdateObjIsNew saves an protobuf message to a key value store after using updateFN to update the message, and returns true if this is a new value.
func UpdateObjIsNew[T proto.Message](ctx context.Context, wf jetstream.KeyValue, k string, msg T, updateFn func(v T) (T, error)) (bool, error) {
	log := logx.FromContext(ctx)
	if log.Enabled(ctx, errors2.TraceLevel) {
		log.Log(ctx, errors2.TraceLevel, "update KV object", slog.String("bucket", wf.Bucket()), slog.String("key", k), slog.Any("fn", reflect.TypeOf(updateFn)))
	}
	isNew := false
	if oldk, err := wf.Get(ctx, k); errors.Is(err, jetstream.ErrKeyNotFound) || (err == nil && oldk.Value() == nil) {
		if err := SaveObj(ctx, wf, k, msg); err != nil {
			return false, fmt.Errorf("save during update object: %w", err)
		}
		isNew = true
	}

	if err := updateKV(ctx, wf, k, msg, func(bv []byte, msg proto.Message) ([]byte, error) {
		if err := proto.Unmarshal(bv, msg); err != nil {
			return nil, fmt.Errorf("unmarshal proto for KV update: %w", err)
		}
		uv, err := updateFn(msg.(T))
		if err != nil {
			return nil, fmt.Errorf("update function: %w", err)
		}
		b, err := proto.Marshal(uv)
		if err != nil {
			return nil, fmt.Errorf("marshal updated proto: %w", err)
		}
		return b, nil
	}); err != nil {
		return false, fmt.Errorf("update obj is new failed: %w", err)
	}
	return isNew, nil
}

// Delete deletes an item from a key value store.
func Delete(ctx context.Context, kv jetstream.KeyValue, key string) error {
	if err := kv.Delete(ctx, key); err != nil {
		return fmt.Errorf("delete key: %w", err)
	}
	return nil
}

// EnsureBuckets ensures that a list of key value stores exist
func EnsureBuckets(ctx context.Context, js jetstream.JetStream, storageType jetstream.StorageType, names []string) error {
	for _, i := range names {
		var ttl time.Duration
		if i == messages.KvLock {
			ttl = time.Second * 30
		}
		if i == messages.KvClients {
			ttl = time.Millisecond * 1500
		}
		if err := EnsureBucket(ctx, js, storageType, i, ttl); err != nil {
			return fmt.Errorf("ensure bucket: %w", err)
		}
	}
	return nil
}

// EnsureBucket creates a bucket if it does not exist
func EnsureBucket(ctx context.Context, js jetstream.JetStream, storageType jetstream.StorageType, name string, ttl time.Duration) error {
	if _, err := js.KeyValue(ctx, name); errors.Is(err, jetstream.ErrBucketNotFound) {
		if _, err := js.CreateKeyValue(ctx, jetstream.KeyValueConfig{
			Bucket:  name,
			Storage: storageType,
			TTL:     ttl,
		}); err != nil {
			return fmt.Errorf("ensure buckets: %w", err)
		}
	} else if err != nil {
		return fmt.Errorf("obtain bucket: %w", err)
	}
	return nil
}

// Process processes messages from a nats consumer and executes a function against each one.
func Process(ctx context.Context, js jetstream.JetStream, streamName string, traceName string, closer chan struct{}, subject string, durable string, concurrency int, middleware []middleware.Receive, fn func(ctx context.Context, log *slog.Logger, msg jetstream.Msg) (bool, error), signalFatalErrFn func(ctx context.Context, state *model.WorkflowState, log *slog.Logger), opts ...ProcessOption) error {
	processOpt := &ProcessOpts{}
	for _, i := range opts {
		i.Set(processOpt)
	}
	log := logx.FromContext(ctx)

	receivers := make([]jetstream.MessagesContext, 0, concurrency)

	for i := 0; i < concurrency; i++ {
		// shared js.Consumer and consumer.Messages per unit of concurrency fails
		// shared js.Consumer and shared consumer.Messages works
		// js.Consumer and consumer.Messages per unit of concurrency works
		consumer, err := js.Consumer(ctx, streamName, durable)
		if err != nil {
			return fmt.Errorf("check durable consumer '%s'present: %w", durable, err)
		}
		if durable != "" {
			if !strings.HasPrefix(durable, "ServiceTask_") {
				conInfo, err := consumer.Info(ctx)
				if err != nil || conInfo.Config.Durable == "" {
					return fmt.Errorf("durable consumer '%s' is not explicity configured", durable)
				}
			}
		}
		messagesContext, err := consumer.Messages(
			jetstream.WithMessagesErrOnMissingHeartbeat(false),
			jetstream.PullMaxMessages(1),
		)
		receivers = append(receivers, messagesContext)
		if err != nil {
			return fmt.Errorf("process consumer %w", err)
		}

		// This spawns the goroutine which will asynchronously process the messages as pulled from nats.
		go func() {
			for {
				m, err := messagesContext.Next()
				if err != nil {
					if errors.Is(err, jetstream.ErrMsgIteratorClosed) {
						return
					}
					// Horrible, but this isn't a typed error.  This test just stops the listener printing pointless errors.
					if err.Error() == "nats: Server Shutdown" || err.Error() == "nats: connection closed" {
						continue
					}
					// Log Error
					log.Error("message fetch error", "error", err)
					continue
				}
				ctx, err := header.FromMsgHeaderToCtx(ctx, m.Headers())
				if x := m.Headers().Get(header.SharNamespace); x == "" {
					log.Error("message without namespace", slog.Any("subject", m.Subject))
					if err := m.Ack(); err != nil {
						log.Error("processing failed to ack", "error", err)
					}
					continue
				}
				if err != nil {
					log.Error("get header values from incoming process message", slog.Any("error", &errors2.ErrWorkflowFatal{Err: err}))
					if err := m.Ack(); err != nil {
						log.Error("processing failed to ack", "error", err)
					}
					continue
				}
				//          log.Debug("Process:"+traceName, slog.String("subject", msg[0].Subject))
				if embargo := m.Headers().Get("embargo"); embargo != "" && embargo != "0" {
					e, err := strconv.Atoi(embargo)
					if err != nil {
						log.Error("bad embargo value", "error", err)
						continue
					}
					offset := time.Duration(int64(e) - time.Now().UnixNano())
					if offset > 0 {
						if err := m.NakWithDelay(offset); err != nil {
							log.Warn("nak with delay")
						}
						continue
					}
				}

				executeCtx, executeLog := logx.NatsMessageLoggingEntrypoint(context.Background(), "server", m.Headers())
				executeCtx = header.Copy(ctx, executeCtx)
				executeCtx = subj.SetNS(executeCtx, m.Headers().Get(header.SharNamespace))

				for _, i := range middleware {
					var err error
					if executeCtx, err = i(executeCtx, m); err != nil {
						slog.Error("process middleware", "error", err, "subject", subject, "middleware", reflect.TypeOf(i).Name())
						continue
					}
				}

				ack, err := fn(executeCtx, executeLog, m)
				if err != nil {
					if errors2.IsWorkflowFatal(err) {
						executeLog.Error("workflow fatal error occurred processing function", "error", err)
						var eWfF *errors2.ErrWorkflowFatal
						errors.As(err, &eWfF)
						if signalFatalErrFn != nil && eWfF.State != nil {
							signalFatalErrFn(executeCtx, eWfF.State, executeLog)
						}
						ack = true
					} else {
						wfe := &workflow.Error{}
						if !errors.As(err, wfe) {
							executeLog.Error("processing error", "error", err, "name", traceName)
							if err != nil {
								//TODO: output this to error chan
								log.Error("fetching message metadata")
							}

							if processOpt.BackoffCalc != nil {
								err := processOpt.BackoffCalc(executeCtx, m)
								if err != nil {
									slog.Error("backoff error", "error", err)
								}
								continue
							}
						}
					}
				}
				if ack {
					if err := m.Ack(); err != nil {
						log.Error("processing failed to ack", "error", err)
					}
				} else {
					if err := m.Nak(); err != nil {
						log.Error("processing failed to nak", "error", err)
					}
				}
			}
		}()
	}
	go func() {
		<-closer
		for _, i := range receivers {
			i.Stop()
		}
	}()
	return nil
}

const jsErrCodeStreamWrongLastSequence = 10071

var lockVal = make([]byte, 0)

// Lock ensures a lock on a given ID, it returns true if a lock was granted.
func Lock(ctx context.Context, kv jetstream.KeyValue, lockID string) (bool, error) {
	_, err := kv.Create(ctx, lockID, lockVal)
	if errors.Is(err, jetstream.ErrKeyExists) {
		return false, nil
	} else if err != nil {
		return false, fmt.Errorf("querying lock: %w", err)
	}
	return true, nil
}

// ExtendLock extends the lock past its stale time.
func ExtendLock(ctx context.Context, kv jetstream.KeyValue, lockID string) error {
	v, err := kv.Get(ctx, lockID)
	if errors.Is(err, jetstream.ErrKeyNotFound) {
		return fmt.Errorf("hold lock found no lock: %w", err)
	} else if err != nil {
		return fmt.Errorf("querying lock: %w", err)
	}
	rev := v.Revision()
	_, err = kv.Update(ctx, lockID, lockVal, rev)
	testErr := &nats.APIError{}
	if errors.As(err, &testErr) {
		if testErr.ErrorCode == jsErrCodeStreamWrongLastSequence {
			return nil
		}
	} else if err != nil {
		return fmt.Errorf("extend lock: %w", err)
	}
	return nil
}

// UnLock closes an existing lock.
func UnLock(ctx context.Context, kv jetstream.KeyValue, lockID string) error {
	_, err := kv.Get(ctx, lockID)
	if errors.Is(err, jetstream.ErrKeyNotFound) {
		return fmt.Errorf("unlocking found no lock: %w", err)
	} else if err != nil {
		return fmt.Errorf("unlocking get lock: %w", err)
	}
	if err := kv.Delete(ctx, lockID); err != nil {
		return fmt.Errorf("unlocking: %w", err)
	}
	return nil
}

func largeObjLock(ctx context.Context, lockKV jetstream.KeyValue, key string) error {
	for {
		cErr := ctx.Err()
		if cErr != nil {
			if errors.Is(cErr, context.DeadlineExceeded) {
				return fmt.Errorf("load large timeout: %w", cErr)
			} else if errors.Is(cErr, context.Canceled) {
				return fmt.Errorf("load large cancelled: %w", cErr)
			} else {
				return fmt.Errorf("load large: %w", cErr)
			}
		}

		lock, err := Lock(ctx, lockKV, key)
		if err != nil {
			return fmt.Errorf("load large locking: %w", err)
		}
		if !lock {
			time.Sleep(500 * time.Millisecond)
			continue
		}
		break
	}
	return nil
}

// LoadLarge load a large binary from the object store
func LoadLarge(ctx context.Context, ds jetstream.ObjectStore, mutex jetstream.KeyValue, key string, opt ...jetstream.GetObjectOpt) ([]byte, error) {
	if err := largeObjLock(ctx, mutex, key); err != nil {
		return nil, fmt.Errorf("load large locking: %w", err)
	}
	defer func() {
		if err := UnLock(ctx, mutex, key); err != nil {
			slog.Error("load large unlocking", "error", err.Error())
		}
	}()
	ret, err := ds.GetBytes(ctx, key, opt...)
	if err != nil {
		return nil, fmt.Errorf("load large get: %w", err)
	}
	return ret, nil
}

// SaveLarge saves a large binary from the object store
func SaveLarge(ctx context.Context, ds jetstream.ObjectStore, mutex jetstream.KeyValue, key string, data []byte) error {
	if err := largeObjLock(ctx, mutex, key); err != nil {
		return fmt.Errorf("save large locking: %w", err)
	}
	defer func() {
		if err := UnLock(ctx, mutex, key); err != nil {
			slog.Error("save large unlocking", "error", err.Error())
		}
	}()
	if _, err := ds.PutBytes(ctx, key, data); err != nil {
		return fmt.Errorf("save large put: %w", err)
	}
	return nil
}

// SaveLargeObj save an protobuf message to a document store
func SaveLargeObj(ctx context.Context, ds jetstream.ObjectStore, mutex jetstream.KeyValue, k string, v proto.Message, opt ...nats.ObjectOpt) error {
	log := logx.FromContext(ctx)
	if log.Enabled(ctx, errors2.TraceLevel) {
		log.Log(ctx, errors2.TraceLevel, "save object", slog.String("key", k), slog.Any("val", v))
	}
	b, err := proto.Marshal(v)
	if err == nil {
		return SaveLarge(ctx, ds, mutex, k, b)
	}
	return fmt.Errorf("save object into KV: %w", err)
}

// LoadLargeObj loads a protobuf message from a key value store
func LoadLargeObj(ctx context.Context, ds jetstream.ObjectStore, mutex jetstream.KeyValue, k string, v proto.Message, opt ...jetstream.GetObjectOpt) error {
	log := logx.FromContext(ctx)
	if log.Enabled(ctx, errors2.TraceLevel) {
		log.Log(ctx, errors2.TraceLevel, "load object", slog.String("key", k), slog.Any("val", v))
	}
	doc, err := LoadLarge(ctx, ds, mutex, k, opt...)
	if err != nil {
		return fmt.Errorf("load object %s: %w", k, err)
	}
	if err := proto.Unmarshal(doc, v); err != nil {
		return fmt.Errorf("load large object unmarshal: %w", err)
	}
	return nil
}

// UpdateLargeObj saves an protobuf message to a document store after using updateFN to update the message.
func UpdateLargeObj[T proto.Message](ctx context.Context, ds jetstream.ObjectStore, mutex jetstream.KeyValue, k string, msg T, updateFn func(v T) (T, error)) (T, error) { //nolint
	log := logx.FromContext(ctx)
	if log.Enabled(ctx, errors2.TraceLevel) {
		log.Log(ctx, errors2.TraceLevel, "update object", slog.String("key", k), slog.Any("fn", reflect.TypeOf(updateFn)))
	}

	if err := LoadLargeObj(ctx, ds, mutex, k, msg); err != nil {
		return msg, fmt.Errorf("update large object load: %w", err)
	}
	nw, err := updateFn(msg)
	if err != nil {
		return msg, fmt.Errorf("update large object updateFn: %w", err)
	}
	if err := SaveLargeObj(ctx, ds, mutex, k, msg); err != nil {
		return msg, fmt.Errorf("update large object save: %w", err)
	}
	return nw, nil
}

// DeleteLarge deletes a large binary from the object store
func DeleteLarge(ctx context.Context, ds jetstream.ObjectStore, mutex jetstream.KeyValue, k string) error {
	if err := largeObjLock(ctx, mutex, k); err != nil {
		return fmt.Errorf("delete large locking: %w", err)
	}
	defer func() {
		if err := UnLock(ctx, mutex, k); err != nil {
			slog.Error("load large unlocking", "error", err.Error())
		}
	}()
	if err := ds.Delete(ctx, k); err != nil && errors.Is(err, jetstream.ErrNoObjectsFound) && !errors.Is(err, jetstream.ErrKeyNotFound) {
		return fmt.Errorf("delete large removing: %w", err)
	}
	return nil
}

// PublishOnce sets up a single message to be used as a timer.
func PublishOnce(ctx context.Context, js jetstream.JetStream, lockingKV jetstream.KeyValue, streamName string, consumerName string, msg *nats.Msg) error {
	consumer, err := js.Consumer(ctx, streamName, consumerName)
	if err != nil {
		return fmt.Errorf("get consumer: %w", err)
	}
	cInfo, err := consumer.Info(ctx)
	if err != nil {
		return fmt.Errorf("obtaining publish once consumer information: %w", err)
	}
	if cInfo.NumPending+uint64(cInfo.NumAckPending)+uint64(cInfo.NumWaiting) == 0 { //nolint:gosec
		if lock, err := Lock(ctx, lockingKV, consumerName); err != nil {
			return fmt.Errorf("obtaining lock for publish once consumer: %w", err)
		} else if lock {
			defer func() {
				// clear the lock out of courtesy
				if err := UnLock(ctx, lockingKV, consumerName); err != nil {
					slog.Warn("releasing lock for publish once consumer")
				}
			}()
			if _, err := js.PublishMsg(ctx, msg); err != nil {
				return fmt.Errorf("starting publish once message: %w", err)
			}
		}
	}
	return nil
}

// PublishObj publishes a proto message to a subject.
func PublishObj(ctx context.Context, conn NatsConn, subject string, prot proto.Message, middlewareFn func(*nats.Msg) error) error {
	msg := nats.NewMsg(subject)
	b, err := proto.Marshal(prot)
	if err != nil {
		return fmt.Errorf("serialize proto: %w", err)
	}
	msg.Data = b
	if middlewareFn != nil {
		if err := middlewareFn(msg); err != nil {
			return fmt.Errorf("middleware: %w", err)
		}
	}
	if err = conn.PublishMsg(msg); err != nil {
		return fmt.Errorf("publish message: %w", err)
	}
	return nil
}

// KeyPrefixResultOpts represents the options for KeyPrefixSearch function.
// Sort field indicates whether the returned values should be sorted.
// ExcludeDeleted field filters out deleted key-values from the result.
type KeyPrefixResultOpts struct {
	Sort           bool // Sort the returned values
	ExcludeDeleted bool // ExcludeDeleted filters deleted key-values from the result (cost penalty)¬.
}

// KeyPrefixSearch searches for keys in a key-value store that have a specified prefix.
// It retrieves the keys by querying the JetStream stream associated with the key-value store.
// It returns a slice of strings containing the keys, and an error if any.
func KeyPrefixSearch(ctx context.Context, js jetstream.JetStream, kv jetstream.KeyValue, prefix string, opts KeyPrefixResultOpts) ([]string, error) {
	kvName := kv.Bucket()
	streamName := "KV_" + kvName
	subjectTrim := fmt.Sprintf("$KV.%s.", kvName)
	subjectPrefix := fmt.Sprintf("%s%s.", subjectTrim, prefix)
	kvs, err := js.Stream(ctx, streamName)
	if err != nil {
		return nil, fmt.Errorf("get stream: %w", err)
	}
	nfo, err := kvs.Info(ctx, jetstream.WithSubjectFilter(subjectPrefix+">"))
	if err != nil {
		return nil, fmt.Errorf("get stream info: %w", err)
	}
	ret := make([]string, 0, len(nfo.State.Subjects))
	kvNameSubjectPrefixLength := len(subjectTrim)
	for s := range nfo.State.Subjects {
		if len(s) >= kvNameSubjectPrefixLength {
			ret = append(ret, s[kvNameSubjectPrefixLength:])
		}
	}

	if opts.Sort {
		slices.Sort(ret)
	}
	if opts.ExcludeDeleted {
		var fnErr error
		ret = slices.DeleteFunc(ret, func(k string) bool {
			_, err := kv.Get(ctx, k)
			if err != nil {
				if errors.Is(err, jetstream.ErrKeyNotFound) {
					return true
				} else {
					fnErr = err
					return true
				}
			}
			return false
		})
		if fnErr != nil {
			return nil, fmt.Errorf("get key value: %w", fnErr)
		}
	}
	return ret, nil
}

// KeyPrefixSearchMap searches for keys in a key-value store that have a specified prefix.
// It retrieves the keys by querying the JetStream stream associated with the key-value store.
// It returns a map of strings containing the keys, and an error if any.
func KeyPrefixSearchMap(ctx context.Context, js jetstream.JetStream, kv jetstream.KeyValue, prefix string, opts KeyPrefixResultOpts) (map[string]struct{}, error) {
	kvName := kv.Bucket()
	streamName := "KV_" + kvName
	subjectTrim := fmt.Sprintf("$KV.%s.", kvName)
	subjectPrefix := fmt.Sprintf("%s%s.", subjectTrim, prefix)
	kvs, err := js.Stream(ctx, streamName)
	if err != nil {
		return nil, fmt.Errorf("get stream: %w", err)
	}
	nfo, err := kvs.Info(ctx, jetstream.WithSubjectFilter(subjectPrefix+">"))
	if err != nil {
		return nil, fmt.Errorf("get stream info: %w", err)
	}
	ret := make(map[string]struct{}, len(nfo.State.Subjects))
	trim := len(subjectTrim)
	for s := range nfo.State.Subjects {
		if len(s) >= trim {
			ret[s[trim:]] = struct{}{}
		}
	}

	if opts.Sort {
		return nil, fmt.Errorf("sort specified on map result")
	}
	if opts.ExcludeDeleted {
		var fnErr error
		maps.DeleteFunc(ret, func(k string, v struct{}) bool {
			_, err := kv.Get(ctx, k)
			if err != nil {
				if errors.Is(err, jetstream.ErrKeyNotFound) {
					return true
				} else {
					fnErr = err
					return true
				}
			}
			return false
		})
		if fnErr != nil {
			return nil, fmt.Errorf("get key value: %w", fnErr)
		}
	}
	return ret, nil
}

// CheckVersion checks the NATS server version against a minimum supported version
func CheckVersion(ctx context.Context, nc *nats.Conn) error {
	nvStr := nc.ConnectedServerVersion()
	nv, err := version2.NewVersion(nvStr)
	if err != nil {
		return fmt.Errorf("parse nats version: %w", err)
	}
	if nv.LessThan(version3.NatsVersion) {
		return fmt.Errorf("nats version %s not supported.  The minimum supported version is %s", nvStr, version3.NatsVersion)
	}
	return nil
}

const (
	strErrHeader    = "STR_ERR"
	strCancelHeader = "STR_CANCEL"
	strEOF          = "STR_EOF"
)

// StreamingReplyClient establishes a streaming reply client. It creates a subscription
// for replies and invokes a callback function for each received message.
func StreamingReplyClient(ctx context.Context, nc *nats.Conn, msg *nats.Msg, fn func(msg *nats.Msg) error) error {
	// call
	ctx, ctxCancel := context.WithCancel(ctx)
	ret := make(chan *nats.Msg)
	errs := make(chan error, 1)
	cancel := make(chan struct{})
	msg.Reply = nats.NewInbox()
	cancelInbox := nats.NewInbox()

	// listener
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case <-cancel:
				m := nats.NewMsg(cancelInbox)
				if err := nc.PublishMsg(m); err != nil {
					fmt.Println("send cancel msg: %w", err)
				}
				ctxCancel()
				return
			}
		}
	}()
	// create subscription for replies
	sub, err := nc.Subscribe(msg.Reply, func(m *nats.Msg) {
		if eHdr := m.Header.Get(strErrHeader); eHdr != "" {
			if eHdr != strEOF {
				errs <- fmt.Errorf("%s", eHdr)
			}
			close(errs)
			ctxCancel()
			return
		}
		ret <- m
	})
	if err != nil {
		ctxCancel()
		return fmt.Errorf("subscribe: %s", err)
	}

	defer func() {
		if err := sub.Unsubscribe(); err != nil {
			slog.Error("unsubscribe", "subject", msg.Subject, "error", err)
		}
	}()

	msg.Header.Set(strCancelHeader, cancelInbox)
	if err := nc.PublishMsg(msg); err != nil {
		ctxCancel()
		return fmt.Errorf("publish: %s", err)
	}
	for {
		select {
		case r := <-ret:
			if err := fn(r); err != nil {
				close(cancel)
				if errors.Is(err, ErrStreamCancel) {
					ctxCancel()
					return nil
				} else {
					ctxCancel()
					return fmt.Errorf("StreamingReplyClient client: %w", err)
				}
			}
		case err := <-errs:
			if err != nil {
				ctxCancel()
				return fmt.Errorf("StreamingReplyClient server: %w", err)
			}
			ctxCancel()
			return nil
		}
	}
}

// ErrStreamCancel is an error variable that represents a stream cancellation.
var ErrStreamCancel = errors.New("stream cancelled")

type streamNatsReplyconnection interface {
	QueueSubscribe(subj, queue string, cb nats.MsgHandler) (*nats.Subscription, error)
	PublishMsg(m *nats.Msg) error
	Subscribe(subj string, cb nats.MsgHandler) (*nats.Subscription, error)
}

// StreamingReplyServer is a function that sets up a NATS subscription to handle streaming reply messages.
// When a message is received, it begins streaming by creating channels for return messages and error messages.
// It then executes the provided function to process the request and send the response messages.
// The function runs in a separate goroutine.
// It continuously listens for return messages and error messages, and publishes them to the reply inbox.
// It exits when an error or cancellation occurs.
// The function returns the NATS subscription and any error that occurred during setup.
func StreamingReplyServer(nc streamNatsReplyconnection, subject string, fn func(req *nats.Msg, ret chan *nats.Msg, errs chan error)) (*nats.Subscription, error) {
	sub, err := nc.QueueSubscribe(subject, subject, func(msg *nats.Msg) {
		// Begin streaming
		ret := make(chan *nats.Msg)
		retErr := make(chan error, 1)
		replyInbox := msg.Reply
		cancelInbox := msg.Header.Get(strCancelHeader)
		cSub, err := nc.Subscribe(cancelInbox, func(msg *nats.Msg) {
			retErr <- ErrStreamCancel
		})
		if err != nil {
			slog.Error("subscribe to cancel inbox", "error", err, "subject", cancelInbox)
			return
		}
		defer func() {
			if err := cSub.Unsubscribe(); err != nil {
				slog.Error("unsubscribe from cancel inbox", "error", err, "subject", cancelInbox)
			}
		}()

		go func() {
			fn(msg, ret, retErr)
			close(retErr)
		}()
		exit := false
		for {
			select {
			case r := <-ret:
				r.Subject = replyInbox
				if err := nc.PublishMsg(r); err != nil {
					slog.Error("publish streaming message", "error", err, "subject", replyInbox)
				}
			case e, ok := <-retErr:
				retM := nats.NewMsg(replyInbox)
				if !ok {
					retM.Header.Set(strErrHeader, strEOF)
				} else if errors.Is(e, ErrStreamCancel) {
					exit = true
					slog.Debug("client cancelled stream", "subject", msg.Subject)
					break
				} else {
					retM.Header.Set(strErrHeader, e.Error())
				}
				exit = true
				if err := nc.PublishMsg(retM); err != nil {
					slog.Error("publish error message", "error", err, "subject", replyInbox, "code", retM.Header.Get(strErrHeader))
				}
			}
			if exit {
				break
			}
		}
	})
	if err != nil {
		return nil, fmt.Errorf("queue subscribe: %s", err)
	}
	return sub, nil
}
