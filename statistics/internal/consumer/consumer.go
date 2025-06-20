package consumer

import (
	"context"
	"encoding/json"
	"github.com/misshanya/url-shortener/statistics/internal/models"
	"github.com/segmentio/kafka-go"
	"log/slog"
)

type service interface {
	Shortened(*models.KafkaMessageShortened)
	Unshortened(*models.KafkaMessageUnshortened)
}

type Consumer struct {
	l   *slog.Logger
	kr  *kafka.Reader
	svc service
}

func New(l *slog.Logger, kr *kafka.Reader, svc service) *Consumer {
	return &Consumer{
		l:   l,
		kr:  kr,
		svc: svc,
	}
}

func (c *Consumer) ReadMessages(ctx context.Context) {
	for {
		m, err := c.kr.ReadMessage(ctx)
		if err != nil {
			c.l.Error("Failed to read message", "error", err)
		}

		switch m.Topic {
		case "shortener.shortened":
			var msg models.KafkaMessageShortened
			if err := json.Unmarshal(m.Value, &msg); err != nil {
				c.l.Error("Failed to unmarshal JSON",
					"topic", m.Topic,
					"error", err,
				)
				continue
			}

			c.svc.Shortened(&msg)
		case "shortener.unshortened":
			var msg models.KafkaMessageUnshortened
			if err := json.Unmarshal(m.Value, &msg); err != nil {
				c.l.Error("Failed to unmarshal JSON",
					"topic", m.Topic,
					"error", err,
				)
				continue
			}

			c.svc.Unshortened(&msg)
		}
	}
}
