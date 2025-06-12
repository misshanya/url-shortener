FROM golang:1.24-alpine AS builder

WORKDIR /app

COPY . .

RUN go mod download

ENV GOCACHE=/root/.cache/go-build
RUN --mount=type=cache,target="/root/.cache/go-build" go build -o gateway ./cmd/

FROM alpine:latest AS runner

WORKDIR /app

COPY --from=builder /app/gateway ./gateway

RUN addgroup -S gateway && adduser -S gateway -G gateway
USER gateway

CMD ["./gateway"]