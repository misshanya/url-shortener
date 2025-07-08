package consumer

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/misshanya/url-shortener/shortener/internal/models"
	"github.com/segmentio/kafka-go"
	"go.opentelemetry.io/otel/propagation"
	"log/slog"
)

type service interface {
	SetTop(ctx context.Context, msg *models.KafkaMessageUnshortenedTop)
}

type Consumer struct {
	l          *slog.Logger
	kr         *kafka.Reader
	svc        service
	cachingTTL int
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
			if errors.Is(err, context.Canceled) {
				return
			}
			c.l.Error("Failed to read message", "error", err)
			continue
		}

		if m.Topic != "shortener.top_unshortened" {
			continue
		}

		propagator := propagation.TraceContext{}
		carrier := propagation.MapCarrier{}

		// Get trace context from message headers
		if len(m.Headers) > 0 {
			for _, header := range m.Headers {
				carrier.Set(header.Key, string(header.Value))
			}
		}

		ctxEvent := propagator.Extract(ctx, carrier)

		var msg models.KafkaMessageUnshortenedTop
		if err := json.Unmarshal(m.Value, &msg); err != nil {
			c.l.Error("Failed to unmarshal JSON",
				"topic", m.Topic,
				"error", err)
			continue
		}

		c.svc.SetTop(ctxEvent, &msg)
	}
}
