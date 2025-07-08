package consumer

import (
	"context"
	"encoding/json"
	"github.com/misshanya/url-shortener/statistics/internal/models"
	"github.com/segmentio/kafka-go"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
	"log/slog"
)

type service interface {
	Shortened(context.Context, *models.KafkaMessageShortened)
	Unshortened(context.Context, *models.KafkaMessageUnshortened)
}

type Consumer struct {
	l   *slog.Logger
	kr  *kafka.Reader
	svc service
	t   trace.Tracer
}

func New(l *slog.Logger, kr *kafka.Reader, svc service, t trace.Tracer) *Consumer {
	return &Consumer{
		l:   l,
		kr:  kr,
		svc: svc,
		t:   t,
	}
}

func (c *Consumer) ReadMessages(ctx context.Context) {
	for {
		m, err := c.kr.ReadMessage(ctx)
		if err != nil {
			c.l.Error("Failed to read message", "error", err)
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

		switch m.Topic {
		case "shortener.shortened":
			ctxEvent, spanEvent := c.t.Start(ctxEvent, "Handle shortened event")

			var msg models.KafkaMessageShortened
			if err := json.Unmarshal(m.Value, &msg); err != nil {
				c.l.Error("Failed to unmarshal JSON",
					"topic", m.Topic,
					"error", err,
				)
				continue
			}

			c.svc.Shortened(ctxEvent, &msg)

			spanEvent.End()
		case "shortener.unshortened":
			ctxEvent, spanEvent := c.t.Start(ctxEvent, "Handle unshortened event")

			var msg models.KafkaMessageUnshortened
			if err := json.Unmarshal(m.Value, &msg); err != nil {
				c.l.Error("Failed to unmarshal JSON",
					"topic", m.Topic,
					"error", err,
				)
				continue
			}

			c.svc.Unshortened(ctxEvent, &msg)

			spanEvent.End()
		}
	}
}
