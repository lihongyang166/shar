package natz

import (
	"context"
	_ "embed"
	"fmt"
	"github.com/goccy/go-yaml"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
	"gitlab.com/shar-workflow/shar/common"
	"gitlab.com/shar-workflow/shar/common/namespace"
	"gitlab.com/shar-workflow/shar/common/setup"
	"gitlab.com/shar-workflow/shar/server/messages"
	"sync"
)

// NatsConfig holds the current nats configuration for SHAR.
//
//go:embed nats-config.yaml
var NatsConfig string

// NatsConnConfiguration represents the configuration for a NATS connection.
//
// - Conn: The NATS connection.
// - TxConn: The transactional NATS connection.
// - StorageType: The storage type for JetStream.
type NatsConnConfiguration struct {
	Conn        *nats.Conn
	TxConn      *nats.Conn
	StorageType jetstream.StorageType
}

// NatsService contains items enabling nats related communications e.g. publish, nats object manipulation
// via jetstream and namespaced KV access.
type NatsService struct {
	Js          jetstream.JetStream
	TxJS        jetstream.JetStream
	Conn        common.NatsConn
	txConn      common.NatsConn
	StorageType jetstream.StorageType
	sharKvs     map[string]*NamespaceKvs
	Rwmx        sync.RWMutex
}

// NamespaceKvs defines all of the key value stores shar needs to operate
type NamespaceKvs struct {
	WfExecution       jetstream.KeyValue
	WfProcessInstance jetstream.KeyValue
	WfUserTasks       jetstream.KeyValue
	WfVarState        jetstream.KeyValue
	WfTaskSpec        jetstream.KeyValue
	WfTaskSpecVer     jetstream.KeyValue
	Wf                jetstream.KeyValue
	WfVersion         jetstream.KeyValue
	WfTracking        jetstream.KeyValue
	Job               jetstream.KeyValue
	OwnerName         jetstream.KeyValue
	OwnerID           jetstream.KeyValue
	WfClientTask      jetstream.KeyValue
	WfGateway         jetstream.KeyValue
	WfName            jetstream.KeyValue
	WfHistory         jetstream.KeyValue
	WfLock            jetstream.KeyValue
	WfMsgTypes        jetstream.KeyValue
	WfProcess         jetstream.KeyValue
	WfMessages        jetstream.KeyValue
	WfClients         jetstream.KeyValue
}

// NewNatsService constructs a new NatsService
func NewNatsService(nc *NatsConnConfiguration) (*NatsService, error) {
	namespace := namespace.Default

	js, err := jetstream.New(nc.Conn)
	if err != nil {
		return nil, fmt.Errorf("open jetstream: %w", err)
	}
	txJS, err := jetstream.New(nc.TxConn)
	if err != nil {
		return nil, fmt.Errorf("open jetstream: %w", err)
	}

	ctx := context.Background()
	if err := setup.Nats(ctx, nc.Conn, js, nc.StorageType, NatsConfig, true, namespace); err != nil {
		return nil, fmt.Errorf("set up nats queue insfrastructure: %w", err)
	}

	nKvs, err2 := initNamespacedKvs(ctx, namespace, js, nc.StorageType, NatsConfig)
	if err2 != nil {
		return nil, fmt.Errorf("failed to init kvs for ns %s, %w", namespace, err2)
	}

	return &NatsService{
		Js:          js,
		TxJS:        txJS,
		Conn:        nc.Conn,
		txConn:      nc.TxConn,
		StorageType: nc.StorageType,
		sharKvs:     map[string]*NamespaceKvs{namespace: nKvs},
	}, nil

}

// KvsFor retrieves the shar KVs for a given namespace. If they do not exist for a namespace,
// it will initialise them and store them in a map for future lookup.
func (s *NatsService) KvsFor(ctx context.Context, ns string) (*NamespaceKvs, error) {
	s.Rwmx.RLock()
	if nsKvs, exists := s.sharKvs[ns]; !exists {
		s.Rwmx.RUnlock()
		s.Rwmx.Lock()
		kvs, err := initNamespacedKvs(ctx, ns, s.Js, s.StorageType, NatsConfig)
		if err != nil {
			s.Rwmx.Unlock()
			return nil, fmt.Errorf("failed to initialise KVs for namespace %s: %w", ns, err)
		}

		s.sharKvs[ns] = kvs
		s.Rwmx.Unlock()
		return kvs, nil
	} else {
		s.Rwmx.RUnlock()
		return nsKvs, nil
	}
}

func initNamespacedKvs(ctx context.Context, ns string, js jetstream.JetStream, storageType jetstream.StorageType, config string) (*NamespaceKvs, error) {
	cfg := &setup.NatsConfig{}
	if err := yaml.Unmarshal([]byte(config), cfg); err != nil {
		return nil, fmt.Errorf("initNamespacedKvs - parse nats-config.yaml: %w", err)
	}
	err := setup.EnsureBuckets(ctx, cfg, js, storageType, ns)
	if err != nil {
		return nil, fmt.Errorf("initNamespacedKvs - EnsureBuckets: %w", err)
	}

	nKvs := NamespaceKvs{}
	kvs := make(map[string]*jetstream.KeyValue)

	kvs[namespace.PrefixWith(ns, messages.KvWfName)] = &nKvs.WfName
	kvs[namespace.PrefixWith(ns, messages.KvExecution)] = &nKvs.WfExecution
	kvs[namespace.PrefixWith(ns, messages.KvTracking)] = &nKvs.WfTracking
	kvs[namespace.PrefixWith(ns, messages.KvDefinition)] = &nKvs.Wf
	kvs[namespace.PrefixWith(ns, messages.KvJob)] = &nKvs.Job
	kvs[namespace.PrefixWith(ns, messages.KvVersion)] = &nKvs.WfVersion
	kvs[namespace.PrefixWith(ns, messages.KvUserTask)] = &nKvs.WfUserTasks
	kvs[namespace.PrefixWith(ns, messages.KvOwnerID)] = &nKvs.OwnerID
	kvs[namespace.PrefixWith(ns, messages.KvOwnerName)] = &nKvs.OwnerName
	kvs[namespace.PrefixWith(ns, messages.KvClientTaskID)] = &nKvs.WfClientTask
	kvs[namespace.PrefixWith(ns, messages.KvVarState)] = &nKvs.WfVarState
	kvs[namespace.PrefixWith(ns, messages.KvProcessInstance)] = &nKvs.WfProcessInstance
	kvs[namespace.PrefixWith(ns, messages.KvGateway)] = &nKvs.WfGateway
	kvs[namespace.PrefixWith(ns, messages.KvHistory)] = &nKvs.WfHistory
	kvs[namespace.PrefixWith(ns, messages.KvLock)] = &nKvs.WfLock
	kvs[namespace.PrefixWith(ns, messages.KvMessageTypes)] = &nKvs.WfMsgTypes
	kvs[namespace.PrefixWith(ns, messages.KvTaskSpec)] = &nKvs.WfTaskSpec
	kvs[namespace.PrefixWith(ns, messages.KvTaskSpecVersions)] = &nKvs.WfTaskSpecVer
	kvs[namespace.PrefixWith(ns, messages.KvProcess)] = &nKvs.WfProcess
	kvs[namespace.PrefixWith(ns, messages.KvMessages)] = &nKvs.WfMessages
	kvs[namespace.PrefixWith(ns, messages.KvClients)] = &nKvs.WfClients

	for k, v := range kvs {
		kv, err := js.KeyValue(ctx, k)
		if err != nil {
			return nil, fmt.Errorf("open %s KV: %w", k, err)
		}
		*v = kv
	}
	return &nKvs, nil
}
