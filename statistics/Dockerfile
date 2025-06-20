FROM golang:1.24-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./

RUN go mod download

COPY . .

ENV GOCACHE=/root/.cache/go-build
RUN --mount=type=cache,target="/root/.cache/go-build" go build -o statistics ./cmd/

FROM alpine:latest AS runner

WORKDIR /app

COPY --from=builder /app/statistics ./statistics

RUN addgroup -S statistics && adduser -S statistics -G statistics
USER statistics

CMD ["./statistics"]