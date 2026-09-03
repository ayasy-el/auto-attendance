# Build stage
FROM golang:1.23-alpine AS builder

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/auto-attendance ./cmd/attendance

# Runtime stage
FROM alpine:3.22

RUN addgroup -S app && adduser -S -G app app \
    && apk add --no-cache ca-certificates tzdata

WORKDIR /app
COPY --from=builder /out/auto-attendance /app/auto-attendance

USER app
ENTRYPOINT ["/app/auto-attendance"]
CMD ["-config", "/app/config.yaml"]
