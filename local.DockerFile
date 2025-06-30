# syntax=docker/dockerfile:1
# ─────────────────────────────────────────────
# Stage 1: builder – download modules & compile
FROM golang:1.23.5-alpine AS builder
LABEL stage=builder

# Optional dev tooling. Feel free to remove git if you do not run "go get" inside the container.
RUN apk add --no-cache git

# Speed up CGO builds on Apple Silicon -> Linux cross-compile
ENV CGO_ENABLED=0 GOOS=linux GOARCH=amd64

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download

# Copy the rest of the source *after* the module cache step
COPY . .
RUN go build -o /out/server ./cmd/server

# ─────────────────────────────────────────────
# Stage 2: runner – keep Go installed for live-debug, migrate etc.
FROM golang:1.23.5-alpine AS runner
WORKDIR /app

RUN apk add --no-cache docker-cli

# Copy binary (+0.0 MB layer if unchanged) and runtime assets
COPY --from=builder /out/server               ./server
COPY --from=builder /src/internal/config      ./internal/config
COPY --from=builder /src/db/migrations        ./db/migrations

# Expose whatever port your YAML/ENV expects
EXPOSE 8080

# “Debug” builds are noisy; keep it flexible
ENV APP_ENVIRONMENT=local
CMD ["./server"]