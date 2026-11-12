package redis

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	goredis "github.com/redis/go-redis/v9"
)

const deadLetterSuffix = ":dlq"

// Producer sends messages to a Redis Stream.
type Producer struct {
	rdb    *goredis.Client
	stream string
}

// NewProducer returns a Producer for the given stream.
func (c *Client) NewProducer(stream string) *Producer {
	return &Producer{rdb: c.rdb, stream: stream}
}

// Send appends values to the stream. Returns the message ID.
func (p *Producer) Send(ctx context.Context, values map[string]any) (string, error) {
	id, err := p.rdb.XAdd(ctx, &goredis.XAddArgs{
		Stream: p.stream,
		Values: values,
	}).Result()
	if err != nil {
		return "", fmt.Errorf("stream send: %w", err)
	}
	return id, nil
}

// ConsumerConfig configures a stream consumer group.
type ConsumerConfig struct {
	Stream    string
	Group     string
	Consumer  string
	BatchSize int64
	Block     time.Duration
	MaxRetry  int64
}

// Consumer reads from a stream consumer group with retry and dead-letter support.
type Consumer struct {
	rdb *goredis.Client
	cfg ConsumerConfig
	log *slog.Logger
}

// NewConsumer creates the consumer group if it doesn't exist and returns a Consumer.
func (c *Client) NewConsumer(ctx context.Context, cfg ConsumerConfig, logger *slog.Logger) (*Consumer, error) {
	err := c.rdb.XGroupCreateMkStream(ctx, cfg.Stream, cfg.Group, "0").Err()
	if err != nil && err.Error() != "BUSYGROUP Consumer Group name already exists" {
		return nil, fmt.Errorf("create consumer group: %w", err)
	}
	return &Consumer{rdb: c.rdb, cfg: cfg, log: logger}, nil
}

// Message is a decoded stream message.
type Message struct {
	ID     string
	Values map[string]any
}

// Read fetches up to BatchSize pending or new messages.
func (con *Consumer) Read(ctx context.Context) ([]Message, error) {
	res, err := con.rdb.XReadGroup(ctx, &goredis.XReadGroupArgs{
		Group:    con.cfg.Group,
		Consumer: con.cfg.Consumer,
		Streams:  []string{con.cfg.Stream, ">"},
		Count:    con.cfg.BatchSize,
		Block:    con.cfg.Block,
	}).Result()
	if err == goredis.Nil {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("stream read: %w", err)
	}
	var msgs []Message
	for _, s := range res {
		for _, m := range s.Messages {
			msgs = append(msgs, Message{ID: m.ID, Values: m.Values})
		}
	}
	return msgs, nil
}

// Ack acknowledges successful processing of a message.
func (con *Consumer) Ack(ctx context.Context, id string) error {
	return con.rdb.XAck(ctx, con.cfg.Stream, con.cfg.Group, id).Err()
}

// Retry re-queues stale pending messages; moves those exceeding MaxRetry to DLQ.
func (con *Consumer) Retry(ctx context.Context) error {
	pending, err := con.rdb.XPendingExt(ctx, &goredis.XPendingExtArgs{
		Stream: con.cfg.Stream,
		Group:  con.cfg.Group,
		Start:  "-",
		End:    "+",
		Count:  100,
	}).Result()
	if err != nil {
		return fmt.Errorf("pending: %w", err)
	}

	for _, p := range pending {
		if p.RetryCount >= con.cfg.MaxRetry {
			// Move to dead-letter stream
			dlq := con.cfg.Stream + deadLetterSuffix
			err := con.rdb.XAdd(ctx, &goredis.XAddArgs{
				Stream: dlq,
				Values: map[string]any{"original_id": p.ID, "retries": p.RetryCount},
			}).Err()
			if err != nil {
				con.log.Error("dlq write failed", "id", p.ID, "error", err)
				continue
			}
			_ = con.Ack(ctx, p.ID)
			con.log.Warn("message moved to DLQ", "id", p.ID, "retries", p.RetryCount)
			continue
		}
		// Claim and re-queue
		_, err := con.rdb.XClaim(ctx, &goredis.XClaimArgs{
			Stream:   con.cfg.Stream,
			Group:    con.cfg.Group,
			Consumer: con.cfg.Consumer,
			MinIdle:  time.Minute,
			Messages: []string{p.ID},
		}).Result()
		if err != nil {
			con.log.Error("xclaim failed", "id", p.ID, "error", err)
		}
	}
	return nil
}
