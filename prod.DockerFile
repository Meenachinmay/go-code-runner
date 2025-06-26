# syntax=docker/dockerfile:1
###########################
# Stage 1 – builder
###########################
FROM golang:1.23.5-alpine AS builder
LABEL stage=builder

ENV CGO_ENABLED=0 GOOS=linux GOARCH=amd64

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download

COPY . .
# Optional: embed version/commit/date into the binary
ARG BUILD_VERSION=dev
ARG BUILD_COMMIT=unknown
ARG BUILD_TIME=unknown
RUN go build \
    -ldflags "-s -w \
      -X 'main.Version=${BUILD_VERSION}' \
      -X 'main.Commit=${BUILD_COMMIT}' \
      -X 'main.BuildTime=${BUILD_TIME}'" \
    -o /out/server ./cmd/server

###########################
# Stage 2 – minimal runner
###########################
FROM alpine:3.20

# Create non-root user
RUN adduser -D -g '' app
USER app
WORKDIR /app

# Copy artefacts
COPY --from=builder /out/server           ./server
COPY --from=builder /src/internal/config  ./internal/config
COPY --from=builder /src/db/migrations    ./db/migrations

EXPOSE 8080
ENV GIN_MODE=release \
    APP_ENVIRONMENT=prod

ENTRYPOINT ["./server"]