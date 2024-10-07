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

type IndexOptions struct {
	IndexToDisk bool
	Path        string
	WarmupDelay time.Duration
	Ready       func()
}

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
	waitOnce  sync.Once
	started   time.Time
	wait      time.Duration
	ready     func()
}

type IndexFn func(entry jetstream.KeyValueEntry) [][]byte

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

	idx := &Index{
		db:      db,
		closer:  make(<-chan struct{}),
		watcher: watcher,
		indexFn: indexFn,
		buffer:  make(chan jetstream.KeyValueEntry, 256),
		delta:   make(chan jetstream.KeyValueEntry, 2048),
		kv:      kv,
		wait:    options.WarmupDelay,
		ready:   options.Ready,
	}
	return idx, err
}

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
			select {
			case <-time.After(idx.wait):
				idx.ready()
			}
		}()
	}
	go func() {
		for {
			select {
			case u := <-idx.watcher.Updates():
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
		}
	}()
	return nil
}

func (idx *Index) Fetch(prefix []byte) (chan []byte, error) {
	since := time.Since(idx.started)
	if since < idx.wait {
		time.Sleep(idx.wait - time.Since(idx.started))
	}
	ret := make(chan []byte)
	go func() {
		defer close(ret)
		opts := badger.DefaultIteratorOptions
		txn := idx.db.NewTransaction(false)
		defer txn.Discard()
		it := txn.NewIterator(opts)
		defer it.Close()
		for it.Seek(prefix); it.ValidForPrefix(prefix); it.Next() {
			if err := it.Item().Value(func(val []byte) error {
				ret <- val
				return nil
			}); err != nil {
				slog.Error("get index value", "error", err, "key", string(it.Item().Key()))
			}
		}
	}()
	return ret, nil
}
