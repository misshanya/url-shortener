FROM golang:1.24-alpine AS builder

WORKDIR /app

COPY . .

RUN go mod download

ENV GOCACHE=/root/.cache/go-build
RUN --mount=type=cache,target="/root/.cache/go-build" go build -o shortener ./cmd/

FROM alpine:latest AS runner

WORKDIR /app

COPY --from=builder /app/shortener ./shortener

RUN addgroup -S shortener && adduser -S shortener -G shortener
USER shortener

CMD ["./shortener"]