package publisher

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
)

type Publisher struct {
	conn  *amqp.Connection
	ch    *amqp.Channel
	queue string
}

type FetchMessage struct {
	PMID    string `json:"pmid"`
	QueryID int64  `json:"query_id"`
}

func New(url, queue string) (*Publisher, error) {
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

	if _, err := ch.QueueDeclare(queue, true, false, false, false, nil); err != nil {
		ch.Close()
		conn.Close()
		return nil, fmt.Errorf("queue declare: %w", err)
	}

	return &Publisher{conn: conn, ch: ch, queue: queue}, nil
}

func (p *Publisher) Publish(ctx context.Context, msg FetchMessage) error {
	body, err := json.Marshal(msg)
	if err != nil {
		return err
	}
	pubCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	return p.ch.PublishWithContext(pubCtx, "", p.queue, false, false, amqp.Publishing{
		ContentType:  "application/json",
		DeliveryMode: amqp.Persistent,
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
