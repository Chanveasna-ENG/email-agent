# Build stage
FROM golang:alpine AS builder

WORKDIR /src

# Install git and ca-certificates for fetching dependencies if needed
RUN apk add --no-cache ca-certificates tzdata

COPY go.mod go.sum ./
RUN go mod download

COPY . .

# Build static binary without CGO (modernc.org/sqlite is pure Go)
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /bin/email-agent ./cmd/email-agent

# Runtime stage: Distroless static with ca-certificates and tzdata
FROM gcr.io/distroless/static-debian12:nonroot

WORKDIR /app

# Copy binary from builder
COPY --from=builder /bin/email-agent /app/email-agent

# Copy default personas and system prompts
COPY --from=builder /src/personas /app/personas

# Set user to non-root
USER nonroot:nonroot

# Persistent data volume for SQLite database and attachments
VOLUME ["/app/data"]

ENTRYPOINT ["/app/email-agent"]
