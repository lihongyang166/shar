package usertaskindexer

import (
	"context"
	"errors"
	"fmt"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
	"gitlab.com/shar-workflow/shar/common/indexer"
	"gitlab.com/shar-workflow/shar/common/middleware"
	"gitlab.com/shar-workflow/shar/model"
	"google.golang.org/protobuf/proto"
	"log/slog"
	"strings"
	"sync"
	"time"
)

// UserTaskIndexer is responsible for indexing and querying user tasks within a JetStream context.
type UserTaskIndexer struct {
	closer     chan struct{}
	js         jetstream.JetStream
	mw         []middleware.Receive
	nc         *nats.Conn
	indexer    map[string]*indexer.Index
	consumeCtx jetstream.ConsumeContext
	watcherMx  sync.Mutex
	warmup     time.Duration
	readyFn    func()
}

const userTaskOps = "WORKFLOW.*.State.Job.Execute.UserTask"

// IndexNotReadyErr is returned when an operation is attempted on an index that has not completed initialization.
var IndexNotReadyErr = errors.New("index not ready")

// IndexOptions holds configuration options for the indexing process.
type IndexOptions struct {
	IndexToDisk bool          // IndexToDisk creates an index on disk if true.
	Path        string        // Path on disk for the index files.  This must be ephemeral, or externally deleted before calling Start().
	WarmupDelay time.Duration // WarmupDelay is the time that the index waits before allowing queries.
	Ready       func()        // Ready function is called once the warmup delay has passed.
}

// New initializes a new UserTaskIndexer, connecting to JetStream and Badger and setting up middleware and consumers.
func New(nc *nats.Conn, options IndexOptions) (*UserTaskIndexer, error) {
	// Connect to JetStream
	js, err := jetstream.New(nc)
	if err != nil {
		return nil, fmt.Errorf("connect to jetstream: %w", err)
	}

	// Set up middleware
	mw := make([]middleware.Receive, 0)

	idx := &UserTaskIndexer{
		nc:      nc,
		js:      js,
		closer:  make(chan struct{}),
		mw:      mw,
		indexer: make(map[string]*indexer.Index),
		readyFn: options.Ready,
		warmup:  options.WarmupDelay,
	}

	cfg := jetstream.ConsumerConfig{FilterSubject: userTaskOps}

	ctx := context.Background()
	consumer, err := js.CreateConsumer(ctx, "WORKFLOW", cfg)
	if err != nil {
		return nil, fmt.Errorf("create consumer: %w", err)
	}
	consumeCtx, err := consumer.Consume(func(msg jetstream.Msg) {
		ns := strings.Split(msg.Subject(), ".")[1]
		kvName := ns + "_WORKFLOW_USERTASK"
		idx.watcherMx.Lock()
		if _, ok := idx.indexer[ns]; !ok {
			kv, err := idx.js.KeyValue(ctx, kvName)
			if err != nil {
				slog.ErrorContext(ctx, "get key value", "error", err)
			}
			kvIndex, err := indexer.New(ctx, kv, &indexer.IndexOptions{}, idx.indexFunc)
			if err != nil {
				slog.ErrorContext(ctx, "create indexer", "error", err)
			}
			idx.indexer[kvName] = kvIndex
			if err := kvIndex.Start(); err != nil {
				slog.ErrorContext(ctx, "start indexer", "error", err)
			}
		}
		idx.watcherMx.Unlock()
	})
	if err != nil {
		return nil, fmt.Errorf("start watcher: %w", err)
	}
	idx.consumeCtx = consumeCtx
	return idx, nil
}

func (idx *UserTaskIndexer) indexFunc(kvName string, entry jetstream.KeyValueEntry) [][]byte {
	ns := kvName[:strings.IndexRune(kvName, '_')]
	state := &model.WorkflowState{}
	err := proto.Unmarshal(entry.Value(), state)
	if err != nil {
		slog.Error("unmarshal workflow state: %w", "error", err)
	}
	idxKeys := make([][]byte, 0)
	for _, u := range state.Owners {
		idxKeys = append(idxKeys, []byte(ns+"\u001CU\u001C"+u+"\u001C"+entry.Key()))
	}
	for _, g := range state.Groups {
		idxKeys = append(idxKeys, []byte(ns+"\u001CG\u001C"+g+"\u001C"+entry.Key()))
	}
	fmt.Println("INDEXED:", idxKeys)
	return idxKeys
}

// Start initializes and starts all indexers for user tasks.
func (idx *UserTaskIndexer) Start(ctx context.Context) error {
	idx.watcherMx.Lock()
	for k, v := range idx.indexer {
		if err := v.Start(); err != nil {
			slog.ErrorContext(ctx, "start indexer", "kv", k, "error", err)
			idx.watcherMx.Unlock()
			return fmt.Errorf("start indexer: %w", err)
		}
	}
	idx.watcherMx.Unlock()
	go func() {
		time.Sleep(idx.warmup)
		if idx.readyFn != nil {
			idx.readyFn()
		}
	}()
	kvLister := idx.js.KeyValueStoreNames(ctx)
	if kvLister.Error() != nil {
		slog.Error("get kv store names", "error", kvLister.Error())
		panic(kvLister.Error())
	}
	for name := range kvLister.Name() {
		s := strings.Split(name, "_")
		if len(s) < 2 {
			continue
		}
		if s[1] == "WORKFLOW" && s[2] == "USERTASK" {
			kv, err := idx.js.KeyValue(ctx, name)
			if err != nil {
				return fmt.Errorf("get kv store: %w", err)
			}
			idx.watcherMx.Lock()
			ix, err := indexer.New(ctx, kv, &indexer.IndexOptions{}, idx.indexFunc)
			if err != nil {
				idx.watcherMx.Unlock()
				return fmt.Errorf("create indexer: %w", err)
			}
			idx.indexer[name] = ix
			idx.watcherMx.Unlock()
			if err := ix.Start(); err != nil {
				return fmt.Errorf("start indexer: %w", err)
			}
		}
	}
	return nil
}

// QueryByUser queries user tasks assigned to a specific user within a given namespace.
func (idx *UserTaskIndexer) QueryByUser(ctx context.Context, namespace string, user string, opts ...indexer.FetchOptsFn) (chan *model.WorkflowState, chan error) {
	return idx.queryBy(ctx, namespace, "U", user, opts...)
}

// QueryByGroup queries user tasks assigned to a specific group within a given namespace.
func (idx *UserTaskIndexer) QueryByGroup(ctx context.Context, namespace string, user string, opts ...indexer.FetchOptsFn) (chan *model.WorkflowState, chan error) {
	return idx.queryBy(ctx, namespace, "G", user, opts...)
}

func (idx *UserTaskIndexer) queryBy(ctx context.Context, namespace string, subject, term string, opts ...indexer.FetchOptsFn) (chan *model.WorkflowState, chan error) {
	idx.watcherMx.Lock()
	index, ok := idx.indexer[namespace+"_WORKFLOW_USERTASK"]
	idx.watcherMx.Unlock()
	retErr := make(chan error, 1)
	ret := make(chan *model.WorkflowState, 1)
	if !ok {
		close(retErr)
		close(ret)
		return ret, retErr
	}
	res, errs := index.Fetch(ctx, []byte(namespace+"\u001C"+subject+"\u001C"+term+"\u001C"), opts...)
	go func() {
		for done := false; !done; {
			select {
			case <-ctx.Done():
				retErr <- ctx.Err()
				close(retErr)
				done = true
			case err := <-errs:
				if err == nil {
					close(retErr)
					done = true
				} else {
					retErr <- err
					close(retErr)
					done = true
				}
			case val := <-res:
				state := &model.WorkflowState{}
				err := proto.Unmarshal(val.Value(), state)
				if err != nil {
					retErr <- err
					close(retErr)
					done = true
				}
				ret <- state
			}
		}
	}()
	return ret, retErr
}
