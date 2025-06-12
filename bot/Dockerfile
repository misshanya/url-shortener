FROM golang:1.24-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./

RUN go mod download

COPY . .

ENV GOCACHE=/root/.cache/go-build
RUN --mount=type=cache,target="/root/.cache/go-build" go build -o bot ./cmd/

FROM alpine:latest AS runner

WORKDIR /app

COPY --from=builder /app/bot ./bot

RUN addgroup -S bot && adduser -S bot -G bot
USER bot

CMD ["./bot"]