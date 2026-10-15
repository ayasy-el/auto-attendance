# Build stage
FROM --platform=$BUILDPLATFORM golang:alpine AS builder

ARG TARGETOS
ARG TARGETARCH

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=${TARGETOS:-linux} GOARCH=${TARGETARCH:-amd64} go build -trimpath -ldflags="-s -w" -o /out/auto-attendance ./cmd/attendance

# Runtime stage
FROM alpine:3.22

RUN addgroup -S app && adduser -S -G app app \
    && apk add --no-cache ca-certificates tzdata \
    && mkdir -p /app/logs && chown -R app:app /app

WORKDIR /app
COPY --from=builder --chown=app:app /out/auto-attendance /app/auto-attendance

USER app
ENTRYPOINT ["/app/auto-attendance"]
CMD ["-config", "/app/config.yaml"]
