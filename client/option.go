package client

import "github.com/nats-io/nats.go"

// WithEphemeralStorage specifies a client store the result of all operations in memory.
//
//goland:noinspection GoExportedFuncWithUnexportedType
func WithEphemeralStorage() ephemeralStorage { //nolint
	return ephemeralStorage{}
}

type ephemeralStorage struct{}

func (o ephemeralStorage) configure(client *Client) {
	client.storageType = nats.MemoryStorage
}

// WithConcurrency specifies the number of threads to process each service task.
//
//goland:noinspection GoExportedFuncWithUnexportedType
func WithConcurrency(n int) concurrency { //nolint
	return concurrency{val: n}
}

type concurrency struct {
	val int
}

func (o concurrency) configure(client *Client) {
	client.concurrency = o.val
}

// WithNoRecovery disables panic recovery for debugging.
//
//goland:noinspection GoExportedFuncWithUnexportedType
func WithNoRecovery() noRecovery { //nolint
	return noRecovery{}
}

type noRecovery struct {
}

func (o noRecovery) configure(client *Client) {
	client.noRecovery = true
}

// WithNoOSSig disables SIGINT and SIGKILL processing within the client.
//
//goland:noinspection GoExportedFuncWithUnexportedType
func WithNoOSSig() noOSSig { //nolint
	return noOSSig{}
}

type noOSSig struct {
}

func (o noOSSig) configure(client *Client) {
	client.noOSSig = true
}

// Experimental_WithNamespace **DANGER: EXPERIMENTAL FEATURE.  MAY CAUSE DATA LOSS OR CORRUPTION!!** applies a client namespace.
//
//goland:noinspection GoExportedFuncWithUnexportedType
func Experimental_WithNamespace(name string) namespace { //nolint
	return namespace{name: name}
}

type namespace struct {
	name string
}

func (o namespace) configure(client *Client) {
	client.ns = o.name
}
