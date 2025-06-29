package producer

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/misshanya/url-shortener/statistics/internal/models"
	"github.com/segmentio/kafka-go"
	"log/slog"
	"time"
)

type service interface {
	GetTopUnshortened(ctx context.Context, amount, ttl int) (*models.UnshortenedTop, error)
}

type Producer struct {
	l         *slog.Logger
	svc       service
	kw        *kafka.Writer
	topTTL    int // How old events we want to get in top (in seconds)
	topAmount int // How many events we want to get in top
}

func New(l *slog.Logger, svc service, kw *kafka.Writer, topTTL, topAmount int) *Producer {
	return &Producer{
		l:         l,
		svc:       svc,
		kw:        kw,
		topTTL:    topTTL,
		topAmount: topAmount,
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
	if err := p.kw.WriteMessages(ctx,
		kafka.Message{
			Topic: "shortener.top_unshortened",
			Value: msgMarshaled,
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
			top, err := p.svc.GetTopUnshortened(ctx, p.topAmount, p.topTTL)
			if err != nil {
				p.l.Error("failed to get top of unshortened", "error", err)
				continue
			}

			if len(top.Top) < 1 {
				continue
			}

			p.l.Info("Sending top to Kafka...")
			err = p.sendTopToKafka(ctx, *top)
			if err != nil {
				p.l.Error("failed to send top to Kafka", "error", err)
			}
		case <-ctx.Done():
			return
		}
	}
}
