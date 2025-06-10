package handler

import (
	"context"
	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
	"github.com/google/uuid"
	"log/slog"
	"net/url"
)

type service interface {
	ShortURL(ctx context.Context, url string) (string, error)
}

type Handler struct {
	l *slog.Logger
	s service
}

func New(logger *slog.Logger, svc service) *Handler {
	return &Handler{l: logger, s: svc}
}

func (h *Handler) Default(ctx context.Context, b *bot.Bot, update *models.Update) {
	// If not inline query, answer "Hi"
	if update.InlineQuery == nil {
		if _, err := b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: update.Message.Chat.ID,
			Text:   "Hi",
		}); err != nil {
			h.l.Error("failed to send message", slog.Any("error", err))
		}
		return
	}

	// Log new query
	h.l.Info("new inline query", slog.Any("query", update.InlineQuery))

	// Validate URL
	if _, err := url.ParseRequestURI(update.InlineQuery.Query); err != nil {
		return
	}

	// Short URL
	short, err := h.s.ShortURL(ctx, update.InlineQuery.Query)
	if err != nil {
		return
	}

	// Answer
	if _, err := b.AnswerInlineQuery(ctx, &bot.AnswerInlineQueryParams{
		InlineQueryID: update.InlineQuery.ID,
		Results: []models.InlineQueryResult{
			&models.InlineQueryResultArticle{
				ID:                  uuid.NewString(),
				Title:               "Shorted URL",
				URL:                 short,
				InputMessageContent: models.InputTextMessageContent{MessageText: short},
			},
		},
	}); err != nil {
		h.l.Error("failed to answer inline query", slog.Any("error", err))
	}
}
