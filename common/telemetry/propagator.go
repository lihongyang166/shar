package telemetry

import "github.com/nats-io/nats.go"

type NatsMsgCarrier struct {
	msg *nats.Msg
}

func (c *NatsMsgCarrier) Get(key string) string {
	return c.msg.Header.Get(key)
}
func (c *NatsMsgCarrier) Set(key string, value string) {
	c.msg.Header.Set(key, value)
}
func (c *NatsMsgCarrier) Keys() []string {
	if c.msg.Header == nil {
		return make([]string, 0)
	}
	ret := make([]string, 0, len(c.msg.Header))
	for k, _ := range c.msg.Header {
		ret = append(ret, k)
	}
	return ret
}

func NewNatsMsgCarrier(msg *nats.Msg) *NatsMsgCarrier {
	return &NatsMsgCarrier{
		msg: msg,
	}
}
