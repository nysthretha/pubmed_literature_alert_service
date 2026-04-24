package publisher

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
)

const (
	FetchQueue         = "pmid.fetch"
	ManualTriggerQueue = "digest.manual_trigger"

	// Manual trigger messages are ephemeral — if nobody consumes within 60s,
	// the trigger is stale (user will re-click rather than see a stale digest).
	manualTriggerTTLMs = 60_000
)

type Publisher struct {
	conn *amqp.Connection
	ch   *amqp.Channel
}

type FetchMessage struct {
	PMID    string `json:"pmid"`
	QueryID int64  `json:"query_id"`
}

type TriggerMessage struct {
	TriggeredAt string `json:"triggered_at"`
}

func New(url string) (*Publisher, error) {
	var conn *amqp.Connection
	var err error
	for i := 1; i <= 5; i++ {
		conn, err = amqp.Dial(url)
		if err == nil {
			break
		}
		slog.Warn("rabbitmq dial attempt failed", "attempt", i, "err", err)
		time.Sleep(2 * time.Second)
	}
	if err != nil {
		return nil, fmt.Errorf("dial: %w", err)
	}

	ch, err := conn.Channel()
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("channel: %w", err)
	}

	if _, err := ch.QueueDeclare(FetchQueue, true, false, false, false, nil); err != nil {
		ch.Close()
		conn.Close()
		return nil, fmt.Errorf("declare %s: %w", FetchQueue, err)
	}

	triggerArgs := amqp.Table{"x-message-ttl": int32(manualTriggerTTLMs)}
	if _, err := ch.QueueDeclare(ManualTriggerQueue, false, false, false, false, triggerArgs); err != nil {
		ch.Close()
		conn.Close()
		return nil, fmt.Errorf("declare %s: %w", ManualTriggerQueue, err)
	}

	return &Publisher{conn: conn, ch: ch}, nil
}

func (p *Publisher) PublishFetch(ctx context.Context, msg FetchMessage) error {
	body, err := json.Marshal(msg)
	if err != nil {
		return err
	}
	pubCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	return p.ch.PublishWithContext(pubCtx, "", FetchQueue, false, false, amqp.Publishing{
		ContentType:  "application/json",
		DeliveryMode: amqp.Persistent,
		Body:         body,
	})
}

func (p *Publisher) PublishManualTrigger(ctx context.Context) error {
	body, _ := json.Marshal(TriggerMessage{TriggeredAt: time.Now().UTC().Format(time.RFC3339)})
	pubCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	return p.ch.PublishWithContext(pubCtx, "", ManualTriggerQueue, false, false, amqp.Publishing{
		ContentType:  "application/json",
		DeliveryMode: amqp.Transient,
		Body:         body,
	})
}

func (p *Publisher) Close() {
	if p.ch != nil {
		p.ch.Close()
	}
	if p.conn != nil {
		p.conn.Close()
	}
}

// IsHealthy reports whether the underlying AMQP connection is still open.
// Used by the /healthz endpoint.
func (p *Publisher) IsHealthy() bool {
	return p.conn != nil && !p.conn.IsClosed()
}
