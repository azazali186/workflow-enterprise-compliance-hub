package bus

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/nats-io/nats.go"
)

type natsBus struct {
	conn *nats.Conn
}

func newNATS(url string) (*natsBus, error) {
	conn, err := nats.Connect(url,
		nats.Timeout(3*time.Second),
		nats.MaxReconnects(5),
		nats.ReconnectWait(2*time.Second),
	)
	if err != nil {
		return nil, fmt.Errorf("connect to nats: %w", err)
	}
	return &natsBus{conn: conn}, nil
}

// Publish serializes the Event and publishes it to the NATS subject.
// Payload is encoded as json.RawMessage so subscribers see the original shape.
func (n *natsBus) Publish(subject string, e Event) error {
	raw, err := json.Marshal(e.Payload)
	if err != nil {
		return fmt.Errorf("marshal payload: %w", err)
	}
	out := e
	out.Payload = json.RawMessage(raw)
	data, err := json.Marshal(out)
	if err != nil {
		return fmt.Errorf("marshal event: %w", err)
	}
	return n.conn.Publish(subject, data)
}

func (n *natsBus) Subscribe(subject string, handler func(data []byte)) (func(), error) {
	sub, err := n.conn.Subscribe(subject, func(m *nats.Msg) {
		handler(m.Data)
	})
	if err != nil {
		return nil, err
	}
	if err := n.conn.FlushTimeout(2 * time.Second); err != nil {
		return nil, err
	}
	return func() { _ = sub.Unsubscribe() }, nil
}

func (n *natsBus) Close() error { n.conn.Close(); return nil }
