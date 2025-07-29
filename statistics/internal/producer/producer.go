package producer

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/misshanya/url-shortener/statistics/internal/models"
	"github.com/segmentio/kafka-go"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
	"log/slog"
	"time"
)

type service interface {
	GetTopUnshortened(ctx context.Context, amount, ttl int) (*models.UnshortenedTop, error)
}

type kafkaWriter interface {
	WriteMessages(ctx context.Context, msgs ...kafka.Message) error
}

type Producer struct {
	l         *slog.Logger
	svc       service
	kw        kafkaWriter
	topTTL    int // How old events we want to get in top (in seconds)
	topAmount int // How many events we want to get in top
	t         trace.Tracer
}

func New(l *slog.Logger, svc service, kw kafkaWriter, topTTL, topAmount int, t trace.Tracer) *Producer {
	return &Producer{
		l:         l,
		svc:       svc,
		kw:        kw,
		topTTL:    topTTL,
		topAmount: topAmount,
		t:         t,
	}
}

func (p *Producer) sendTopToKafka(ctx context.Context, top models.UnshortenedTop) error {
	msg := models.KafkaMessageUnshortenedTop{
		ValidUntil: top.ValidUntil,
		Top: []struct {
			OriginalURL string `json:"original_url"`
			ShortCode   string `json:"short_code"`
		}(top.Top),
	}
	msgMarshaled, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("failed to marshal message: %w", err)
	}

	// Inject trace from context into carrier
	carrier := propagation.MapCarrier{}
	propagator := propagation.TraceContext{}
	propagator.Inject(ctx, carrier)

	// Kafka message headers for the trace context
	headers := make([]kafka.Header, len(carrier.Keys()))

	for i, key := range carrier.Keys() {
		headers[i] = kafka.Header{
			Key:   key,
			Value: []byte(carrier.Get(key)),
		}
	}

	if err := p.kw.WriteMessages(ctx,
		kafka.Message{
			Headers: headers,
			Topic:   "shortener.top_unshortened",
			Value:   msgMarshaled,
		}); err != nil {
		return fmt.Errorf("failed to write message to Kafka: %w", err)
	}

	return nil
}

func (p *Producer) ProduceTop(ctx context.Context) {
	ticker := time.NewTicker(time.Duration(p.topTTL) * time.Second)

	for {
		select {
		case <-ticker.C:
			func() {
				ctxTop, spanTop := p.t.Start(ctx, "ProduceTop")
				defer spanTop.End()

				top, err := p.svc.GetTopUnshortened(ctxTop, p.topAmount, p.topTTL)
				if err != nil {
					p.l.Error("failed to get top of unshortened", "error", err)
					return
				}

				if len(top.Top) < 1 {
					return
				}

				p.l.Info("Sending top to Kafka...")
				err = p.sendTopToKafka(ctxTop, *top)
				if err != nil {
					p.l.Error("failed to send top to Kafka", "error", err)
				}
			}()

		case <-ctx.Done():
			return
		}
	}
}
