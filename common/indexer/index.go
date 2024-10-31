package indexer

import (
	"context"
	"errors"
	"fmt"
	"github.com/dgraph-io/badger/v4"
	"github.com/nats-io/nats.go/jetstream"
	"log/slog"
	"sync"
	"time"
)

// IndexOptions holds configuration options for the indexing process.
type IndexOptions struct {
	IndexToDisk bool          // IndexToDisk creates an index on disk if true.
	Path        string        // Path on disk for the index files.  This must be ephemeral, or externally deleted before calling Start().
	WarmupDelay time.Duration // WarmupDelay is the time that the index waits before allowing queries.
	Ready       func()        // Ready function is called once the warmup delay has passed.
}

var db *badger.DB
var dbOnce sync.Once

// Index - A badger prefix based indexer for NATS KV
type Index struct {
	closer    <-chan struct{}
	watcher   jetstream.KeyWatcher
	indexFn   IndexFn
	kv        jetstream.KeyValue
	startMx   sync.Mutex
	startOnce bool
	buffer    chan jetstream.KeyValueEntry
	delta     chan jetstream.KeyValueEntry
	started   time.Time
	wait      time.Duration
	ready     func()
}

// IndexFn takes a KeyValueEntry, and returns a set of badger keys.
type IndexFn func(kvName string, entry jetstream.KeyValueEntry) [][]byte

// New creates a new instance of the indexer.
func New(ctx context.Context, kv jetstream.KeyValue, options *IndexOptions, indexFn IndexFn) (*Index, error) {
	if options == nil {
		options = &IndexOptions{}
	}
	bOptions := badger.DefaultOptions(options.Path).
		WithInMemory(!options.IndexToDisk)
	var err error
	dbOnce.Do(func() {
		db, err = badger.Open(bOptions)
	})
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}
	watcher, err := kv.WatchAll(ctx)
	if err != nil {
		return nil, fmt.Errorf("create watcher: %w", err)
	}

	return &Index{
		closer:  make(<-chan struct{}),
		watcher: watcher,
		indexFn: indexFn,
		buffer:  make(chan jetstream.KeyValueEntry, 256),
		delta:   make(chan jetstream.KeyValueEntry, 2048),
		kv:      kv,
		wait:    options.WarmupDelay,
		ready:   options.Ready,
	}, nil
}

// Start starts the indexer
func (idx *Index) Start() error {
	idx.startMx.Lock()
	if idx.startOnce {
		return nil
	}
	idx.startOnce = true
	idx.startMx.Unlock()
	idx.started = time.Now()
	if idx.ready != nil {
		go func() {
			<-time.After(idx.wait)
			idx.ready()
		}()
	}
	go func() {
		for u := range idx.watcher.Updates() {
			if u == nil {
				time.Sleep(time.Millisecond * 100)
				continue
			}
			switch u.Operation() {
			case jetstream.KeyValueDelete:
				rv, err := idx.kv.History(context.Background(), u.Key())
				if err != nil {
					panic(err)
				}
				last := rv[len(rv)-2]
				indexKeys := idx.indexFn(idx.kv.Bucket(), last)
				if err := func() error {
					txn := db.NewTransaction(true)
					defer txn.Discard()
					for _, key := range indexKeys {
						if err := txn.Delete(key); err != nil {
							return fmt.Errorf("delete key: %w", err)
						}
					}
					if err := txn.Commit(); err != nil {
						return fmt.Errorf("commit deletes: %w", err)
					}
					return nil
				}(); err != nil {
					slog.Error("delete index keys", "error", err)
				}
			case jetstream.KeyValuePut:
				indexKeys := idx.indexFn(idx.kv.Bucket(), u)
				if err := func() error {
					txn := db.NewTransaction(true)
					defer txn.Discard()
					for _, key := range indexKeys {
						if err := txn.Set(key, []byte(u.Key())); err != nil {
							return fmt.Errorf("delete key: %w", err)
						}
					}
					if err := txn.Commit(); err != nil {
						return fmt.Errorf("commit deletes: %w", err)
					}
					return nil
				}(); err != nil {
					slog.Error("delete index keys", "error", err)
				}
			default:
				slog.Error("unhandled operation", "opcode", u.Operation())
			}
		}
	}()
	return nil
}

// KV returns the underlying jetstream.KeyValue interface from the Index.
func (idx *Index) KV() jetstream.KeyValue { //nolint:ireturn
	return idx.kv
}

// FetchOptsFn is a function type that modifies FetchOpts when fetching key-value entries.
type FetchOptsFn func(opts *FetchOpts)

// Skip skips the first n entries when fetching key-value entries.
func Skip(n int) FetchOptsFn {
	return func(opts *FetchOpts) {
		opts.skip = n
	}
}

// Take sets the number of key-value entries to take when fetching.
func Take(n int) FetchOptsFn {
	return func(opts *FetchOpts) {
		opts.take = n
	}
}

// FetchOpts specifies options for fetching key-value entries.
// The `skip` field determines the number of entries to skip.
// The `take` field specifies the number of entries to retrieve.
type FetchOpts struct {
	skip int
	take int
}

// Fetch - fetches all keys with a key prefix
func (idx *Index) Fetch(ctx context.Context, prefix []byte, opts ...FetchOptsFn) (chan jetstream.KeyValueEntry, chan error) {
	applyOpts := FetchOpts{}
	for _, opt := range opts {
		opt(&applyOpts)
	}
	since := time.Since(idx.started)
	if since < idx.wait {
		time.Sleep(idx.wait - time.Since(idx.started))
	}
	ret := make(chan jetstream.KeyValueEntry, 1)
	retErr := make(chan error, 1)
	go func() {
		opts := badger.DefaultIteratorOptions
		txn := db.NewTransaction(false)
		defer txn.Discard()
		it := txn.NewIterator(opts)
		defer it.Close()
		defer close(retErr)
		it.Seek(prefix)
		if applyOpts.skip > 0 {
			for i := 0; i < applyOpts.skip; i++ {
				if it.ValidForPrefix(prefix) {
					it.Next()
				} else {
					break
				}
			}
		}
		result := 0
		for ; it.ValidForPrefix(prefix); it.Next() {
			if applyOpts.take != 0 {
				result++
				if result > applyOpts.take {
					break
				}
			}
			if err := it.Item().Value(func(val []byte) error {
				e, err := idx.kv.Get(ctx, string(val))
				if err != nil {
					if errors.Is(err, jetstream.ErrKeyNotFound) {
						return nil
					}
					errBack := fmt.Errorf("get nats entry: %w", err)
					retErr <- errBack
					return errBack
				}
				ret <- e
				return nil
			}); err != nil {
				retErr <- fmt.Errorf("get index value: %w", err)
			}
		}
	}()
	return ret, retErr
}
