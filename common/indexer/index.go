package indexer

import (
	"context"
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

// Index - A badger prefix based indexer for NATS KV
type Index struct {
	db        *badger.DB
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
type IndexFn func(entry jetstream.KeyValueEntry) [][]byte

// New creates a new instance of the indexer.
func New(ctx context.Context, kv jetstream.KeyValue, options *IndexOptions, indexFn IndexFn) (*Index, error) {
	if options == nil {
		options = &IndexOptions{}
	}
	bOptions := badger.DefaultOptions(options.Path).
		WithInMemory(!options.IndexToDisk)
	db, err := badger.Open(bOptions)
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}

	watcher, err := kv.WatchAll(ctx)
	if err != nil {
		return nil, fmt.Errorf("create watcher: %w", err)
	}
	return &Index{
		db:      db,
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
				indexKeys := idx.indexFn(last)
				if err := func() error {
					txn := idx.db.NewTransaction(true)
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
				indexKeys := idx.indexFn(u)
				if err := func() error {
					txn := idx.db.NewTransaction(true)
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

// Fetch - fetches all keys with a key prefix
func (idx *Index) Fetch(ctx context.Context, prefix []byte) (chan jetstream.KeyValueEntry, error) {
	since := time.Since(idx.started)
	if since < idx.wait {
		time.Sleep(idx.wait - time.Since(idx.started))
	}
	ret := make(chan jetstream.KeyValueEntry)
	go func() {
		defer close(ret)
		opts := badger.DefaultIteratorOptions
		txn := idx.db.NewTransaction(false)
		defer txn.Discard()
		it := txn.NewIterator(opts)
		defer it.Close()
		for it.Seek(prefix); it.ValidForPrefix(prefix); it.Next() {
			if err := it.Item().Value(func(val []byte) error {
				e, err := idx.kv.Get(ctx, string(val))
				if err != nil {
					return fmt.Errorf("get nats entry: %w", err)
				}
				ret <- e
				return nil
			}); err != nil {
				slog.Error("get index value", "error", err, "key", string(it.Item().Key()))
			}
		}
	}()
	return ret, nil
}
