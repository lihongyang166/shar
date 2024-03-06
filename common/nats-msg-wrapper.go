package common

import (
	"context"
	"errors"
	"fmt"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
	"time"
)

type NatsMsgWrapper struct {
	jetstream.Msg
	msg *nats.Msg
}

func NewNatsMsgWrapper(msg *nats.Msg) *NatsMsgWrapper {
	return &NatsMsgWrapper{
		msg: msg,
	}
}

func (w *NatsMsgWrapper) Metadata() (*jetstream.MsgMetadata, error) {
	md, err := w.msg.Metadata()
	if err != nil {
		return nil, fmt.Errorf("get metadata from underlying msg: %w", err)
	}
	return &jetstream.MsgMetadata{
		Sequence: jetstream.SequencePair{
			Consumer: md.Sequence.Consumer,
			Stream:   md.Sequence.Stream,
		},
		NumDelivered: md.NumDelivered,
		NumPending:   md.NumPending,
		Timestamp:    md.Timestamp,
		Stream:       md.Stream,
		Consumer:     md.Consumer,
		Domain:       md.Domain,
	}, nil
}
func (w *NatsMsgWrapper) Data() []byte {
	return w.msg.Data
}
func (w *NatsMsgWrapper) SetData(b []byte) {
	w.msg.Data = b
}
func (w *NatsMsgWrapper) Headers() nats.Header {
	return w.msg.Header
}
func (w *NatsMsgWrapper) Subject() string {
	return w.msg.Subject
}
func (w *NatsMsgWrapper) Reply() string {
	return w.msg.Reply
}
func (w *NatsMsgWrapper) Ack() error {
	return w.msg.Ack()
}
func (w *NatsMsgWrapper) DoubleAck(context.Context) error {
	return errors.New("double ack not allowed")
}
func (w *NatsMsgWrapper) Nak() error {
	return w.Nak()
}
func (w *NatsMsgWrapper) NakWithDelay(delay time.Duration) error {
	return w.NakWithDelay(delay)
}
func (w *NatsMsgWrapper) InProgress() error {
	return w.InProgress()
}
func (w *NatsMsgWrapper) Term() error {
	return w.Term()
}
