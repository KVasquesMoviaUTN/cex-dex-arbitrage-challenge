# Build Stage
FROM golang:1.24 AS builder

WORKDIR /app

# Install dependencies
COPY go.mod go.sum ./
RUN go mod download -x

# Copy source
COPY . .

# Build binary
# CGO_ENABLED=0 for static binary
RUN CGO_ENABLED=0 GOOS=linux go build -o bot cmd/bot/main.go

# Run Stage
FROM gcr.io/distroless/static-debian12

WORKDIR /app

COPY --from=builder /app/bot .

# Expose metrics port
EXPOSE 9090
# Expose WebSocket port
EXPOSE 8080

# Run
CMD ["./bot"]
