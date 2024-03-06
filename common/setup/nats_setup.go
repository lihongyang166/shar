package setup

import (
	"context"
	_ "embed"
	"errors"
	"fmt"
	"github.com/goccy/go-yaml"
	"github.com/hashicorp/go-version"
	"github.com/nats-io/nats.go/jetstream"
	"gitlab.com/shar-workflow/shar/common"
	"gitlab.com/shar-workflow/shar/common/namespace"
	"gitlab.com/shar-workflow/shar/common/setup/upgrader"
	sharVersion "gitlab.com/shar-workflow/shar/common/version"
	"regexp"
)

// NatsConfig is the SHAR NATS configuration format
type NatsConfig struct {
	Streams  []NatsStream   `json:"streams"`
	KeyValue []NatsKeyValue `json:"buckets"`
}

// NatsKeyValue holds information about a NATS Key-Value store (bucket)
type NatsKeyValue struct {
	Config jetstream.KeyValueConfig `json:"nats-config"`
}

// NatsConsumer holds information about a NATS Consumer
type NatsConsumer struct {
	Config jetstream.ConsumerConfig `json:"nats-config"`
}

// NatsStream holds information about a NATS Stream
type NatsStream struct {
	Config    jetstream.StreamConfig `json:"nats-config"`
	Consumers []NatsConsumer         `json:"nats-consumers"`
}

// Nats sets up nats server objects.
func Nats(ctx context.Context, nc common.NatsConn, js jetstream.JetStream, storageType jetstream.StorageType, config string, update bool, ns string) error {
	cfg := &NatsConfig{}
	if err := yaml.Unmarshal([]byte(config), cfg); err != nil {
		return fmt.Errorf("parse nats-config.yaml: %w", err)
	}

	for _, stream := range cfg.Streams {
		stream.Config.Storage = storageType
		if err := EnsureStream(ctx, nc, js, stream.Config, storageType); err != nil {
			return fmt.Errorf("ensure stream: %w", err)
		}
		for _, consumer := range stream.Consumers {
			consumer.Config.MemoryStorage = storageType == jetstream.MemoryStorage
			if err := EnsureConsumer(ctx, js, stream.Config.Name, consumer.Config, update, storageType); err != nil {
				return fmt.Errorf("ensure consumer: %w", err)
			}
		}
	}

	err := EnsureBuckets(ctx, cfg, js, storageType, ns)
	if err != nil {
		return err
	}

	return nil
}

// EnsureConsumer creates a new consumer appending the current semantic version number to the description.  If the consumer exists and has a previous version, it upgrader it.
func EnsureConsumer(ctx context.Context, js jetstream.JetStream, streamName string, consumerConfig jetstream.ConsumerConfig, update bool, storageType jetstream.StorageType) error {
	consumerConfig.MemoryStorage = storageType == jetstream.MemoryStorage
	existingConsumer, exerr := js.Consumer(ctx, streamName, consumerConfig.Durable)
	var exists bool
	var consumerInfo *jetstream.ConsumerInfo
	if errors.Is(exerr, jetstream.ErrConsumerNotFound) {
		// This is fine, we just don't have this consumer yet
	} else if exerr != nil {
		return fmt.Errorf("consumer exists: %w", exerr)
	} else {
		exists = true
		var err error
		consumerInfo, err = existingConsumer.Info(ctx)
		if err != nil {
			return fmt.Errorf("get existing consumer info: %w", err)
		}
	}
	if !exists {
		if consumerConfig.Metadata == nil {
			consumerConfig.Metadata = make(map[string]string)
		}
		consumerConfig.Metadata["shar_version"] = sharVersion.Version
		if _, err := js.CreateConsumer(ctx, streamName, consumerConfig); err != nil {
			return fmt.Errorf("cannot ensure consumer '%s' with subject '%s' : %w", consumerConfig.Name, consumerConfig.FilterSubject, err)
		}
	} else {
		if !update {
			return nil
		}
		if ok := requiresUpgrade(consumerInfo.Config.Metadata["shar_version"], sharVersion.Version); ok {
			if consumerConfig.Metadata == nil {
				consumerConfig.Metadata = make(map[string]string)
			}
			consumerConfig.Metadata["shar_version"] = sharVersion.Version
			_, err := js.UpdateConsumer(ctx, streamName, consumerConfig)
			if err != nil {
				return fmt.Errorf("ensure stream couldn't update the consumer configuration for %s: %w", consumerConfig.Name, err)
			}
		}
	}
	return nil
}

// EnsureStream creates a new stream appending the current semantic version number to the description.  If the stream exists and has a previous version, it upgrader it.
func EnsureStream(ctx context.Context, nc common.NatsConn, js jetstream.JetStream, streamConfig jetstream.StreamConfig, storageType jetstream.StorageType) error {
	streamConfig.Storage = storageType
	var exists bool
	var streamInfo *jetstream.StreamInfo
	stream, serr := js.Stream(ctx, streamConfig.Name)
	if errors.Is(serr, jetstream.ErrStreamNotFound) {
		// This is fine
	} else if serr != nil {
		return fmt.Errorf("get stream: %w", serr)
	} else {
		exists = true
		var err error
		streamInfo, err = stream.Info(ctx)
		if err != nil {
			return fmt.Errorf("get stream info: %w", err)
		}
	}
	if !exists {
		if streamConfig.Metadata == nil {
			streamConfig.Metadata = make(map[string]string)
		}
		streamConfig.Metadata["shar_version"] = sharVersion.Version
		if _, err := js.CreateStream(ctx, streamConfig); err != nil {
			return fmt.Errorf("create stream: %w", err)
		}
	} else {
		if ok := requiresUpgrade(streamInfo.Config.Metadata["shar_version"], sharVersion.Version); ok {
			existingVer := upgradeExpr.FindString(streamInfo.Config.Description)
			if existingVer == "" {
				existingVer = sharVersion.Version
			}
			if err := upgrader.Patch(ctx, existingVer, nc, js); err != nil {
				return fmt.Errorf("ensure stream: %w", err)
			}
			if streamConfig.Metadata == nil {
				streamConfig.Metadata = make(map[string]string)
			}
			streamConfig.Metadata["shar_version"] = sharVersion.Version
			_, err := js.UpdateStream(ctx, streamConfig)
			if err != nil {
				return fmt.Errorf("ensure stream updating stream configuration: %w", err)
			}
		}
	}
	return nil
}

// EnsureBuckets creates a list of buckets if they do not exist
func EnsureBuckets(ctx context.Context, cfg *NatsConfig, js jetstream.JetStream, storageType jetstream.StorageType, ns string) error {
	namespaceBucketNameFn := func(kvCfg *jetstream.KeyValueConfig) {
		kvCfg.Bucket = namespace.PrefixWith(ns, kvCfg.Bucket)
	}

	for i := range cfg.KeyValue {
		if err := EnsureBucket(ctx, js, cfg.KeyValue[i].Config, storageType, namespaceBucketNameFn); err != nil {
			return fmt.Errorf("ensure key-value: %w", err)
		}
	}
	return nil
}

// EnsureBucket creates a bucket if it does not exist
func EnsureBucket(ctx context.Context, js jetstream.JetStream, cfg jetstream.KeyValueConfig, storageType jetstream.StorageType, namespaceBucketNameFn func(*jetstream.KeyValueConfig)) error {
	namespaceBucketNameFn(&cfg)
	cfg.Storage = storageType

	if _, err := js.KeyValue(ctx, cfg.Bucket); errors.Is(err, jetstream.ErrBucketNotFound) {
		if _, err := js.CreateKeyValue(ctx, cfg); err != nil {
			return fmt.Errorf("ensure buckets: %w", err)
		}
	} else if err != nil {
		return fmt.Errorf("obtain bucket: %w", err)
	}
	return nil
}

// upgradeExpr is the version check regex
var upgradeExpr = regexp.MustCompilePOSIX(`([0-9])*\.([0-9])*\.([0-9])*$`)

// requiresUpgrade reads the description on an existing SHAR JetStream object.  It compares this with the running version and returs true if an upgrade is needed.
func requiresUpgrade(description string, newVersion string) bool {

	if v := upgradeExpr.FindString(description); len(v) == 0 {
		return true
	} else {
		v1, err := version.NewVersion(v)
		if err != nil {
			return true
		}
		v2, err := version.NewVersion(newVersion)
		if err != nil {
			return true
		}
		return v2.GreaterThanOrEqual(v1)
	}
}
