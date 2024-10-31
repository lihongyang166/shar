package tests

import (
	"context"
	"fmt"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gitlab.com/shar-workflow/shar/common"
	"gitlab.com/shar-workflow/shar/common/indexer"
	"strconv"
	"testing"
	"time"
)

func Test_Index(t *testing.T) {
	nc, err := nats.Connect(tst.NatsURL)
	require.NoError(t, err)
	js, err := jetstream.New(nc)
	require.NoError(t, err)
	ctx := context.Background()
	kv, err := js.CreateKeyValue(ctx, jetstream.KeyValueConfig{
		Bucket:  "testvals",
		Storage: jetstream.MemoryStorage,
		History: 2, // History must be 2 or above for deleted values to be retrievable
	})
	require.NoError(t, err)
	for i := 0; i < 1000; i++ {
		_, err := kv.Put(ctx, strconv.Itoa(i), []byte("val"+strconv.Itoa(i)))
		require.NoError(t, err)
	}
	idxOptions := &indexer.IndexOptions{
		IndexToDisk: false,
		WarmupDelay: 2 * time.Second,
		Ready: func() {
			fmt.Println("cache ready")
		},
	}
	idx, err := indexer.New(ctx, kv, idxOptions, func(kvName string, entry jetstream.KeyValueEntry) [][]byte {
		keys := make([][]byte, 0, 1)
		keys = append(keys, append([]byte(entry.Key()), []byte("KV")...))
		return keys
	})
	require.NoError(t, err)
	err = idx.Start()
	require.NoError(t, err)
	_, err = kv.Put(ctx, "key", []byte("value"))
	require.NoError(t, err)
	_, err = kv.Put(ctx, "key", []byte("value2"))
	require.NoError(t, err)
	err = common.SafeDelete(ctx, kv, "key")
	require.NoError(t, err)
	err = common.SafeDelete(ctx, kv, "key")
	require.NoError(t, err)
	target := strconv.Itoa(4)
	results, errs := idx.Fetch(ctx, []byte(target))

	res := make([]jetstream.KeyValueEntry, 0, 1000)
	for done := false; !done; {
		select {
		case err := <-errs:
			require.NoError(t, err)
			done = true
		case r := <-results:
			res = append(res, r)
		}
	}
	assert.Len(t, res, 111)
}