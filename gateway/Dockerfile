FROM golang:1.24-alpine AS builder

WORKDIR /app

COPY . .

RUN go mod download
RUN go build -o gateway ./cmd/

FROM alpine:latest AS runner

WORKDIR /app

COPY --from=builder /app/gateway ./gateway

CMD ["./gateway"]