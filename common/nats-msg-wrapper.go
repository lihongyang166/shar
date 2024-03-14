package common

import (
	"context"
	"errors"
	"fmt"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
	"time"
)

// NatsMsgWrapper is a wrapper type that combines the jetstream.Msg and nats.Msg types.
type NatsMsgWrapper struct {
	jetstream.Msg
	msg *nats.Msg
}

// NewNatsMsgWrapper is a function that creates a new instance of NatsMsgWrapper, which is a wrapper type that combines the jetstream.Msg and nats.Msg types.
func NewNatsMsgWrapper(msg *nats.Msg) *NatsMsgWrapper {
	return &NatsMsgWrapper{
		msg: msg,
	}
}

// Metadata is a method that retrieves the metadata from the underlying nats.Msg.
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

// Data is a method that retrieves the data from the underlying nats.Msg.
func (w *NatsMsgWrapper) Data() []byte {
	return w.msg.Data
}

// SetData is a method that sets the data of the underlying nats.Msg.
func (w *NatsMsgWrapper) SetData(b []byte) {
	w.msg.Data = b
}

// Headers is a method that retrieves the headers from the underlying nats.Msg.
func (w *NatsMsgWrapper) Headers() nats.Header {
	return w.msg.Header
}

// Subject is a method that retrieves the subject from the underlying nats.Msg.
func (w *NatsMsgWrapper) Subject() string {
	return w.msg.Subject
}

// Reply is a method that retrieves the reply from the underlying nats.Msg.
func (w *NatsMsgWrapper) Reply() string {
	return w.msg.Reply
}

// Ack is a method that acknowledges the receipt of the NATS message by calling the underlying nats.Msg's Ack method.
// If an error occurs while acknowledging the message, it returns an error with a message indicating the failure.
// Returns nil if the acknowledgement is successful.
func (w *NatsMsgWrapper) Ack() error {
	if err := w.msg.Ack(); err != nil {
		return fmt.Errorf("ack nats msg: %w", err)
	}
	return nil
}

// DoubleAck is a method that simulates a double acknowledgement of the NATS message.
// It returns an error with a message indicating that double ack is not allowed.
func (w *NatsMsgWrapper) DoubleAck(context.Context) error {
	return fmt.Errorf("doubleAck: %w", errors.New("double ack not allowed"))
}

// Nak is a method that nak's the message..
func (w *NatsMsgWrapper) Nak() error {
	if err := w.msg.Nak(); err != nil {
		return fmt.Errorf("nak nats msg: %w", err)
	}
	return nil
}

// NakWithDelay is a method that nak's the message, and will not re-process before delay.
func (w *NatsMsgWrapper) NakWithDelay(delay time.Duration) error {
	if err := w.msg.NakWithDelay(delay); err != nil {
		return fmt.Errorf("nak with delay: %w", err)
	}
	return nil
}

// InProgress is a method that indicates that the message is still in progress.
func (w *NatsMsgWrapper) InProgress() error {
	if err := w.msg.InProgress(); err != nil {
		return fmt.Errorf("inprogress nats msg: %w", err)
	}
	return nil
}

// Term is a method that calls the `Term` method on the underlying `NatsMsgWrapper` instance.
func (w *NatsMsgWrapper) Term() error {
	if err := w.msg.Term(); err != nil {
		return fmt.Errorf("term nats msg: %w", err)
	}
	return nil
}
