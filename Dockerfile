# Build stage
FROM golang:1.23-alpine AS builder

WORKDIR /app

# Install dependencies for CGO (if needed)
RUN apk add --no-cache gcc musl-dev

# Copy go mod files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Build binary
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o main .

# Final stage
FROM alpine:3.20

WORKDIR /app

# Install ca-certificates for HTTPS requests
RUN apk --no-cache add ca-certificates tzdata

# Set timezone
ENV TZ=Asia/Jakarta

# Copy binary from builder
COPY --from=builder /app/main .

# Copy migration files
COPY --from=builder /app/migration ./migration

# Expose port
EXPOSE 3000

# Run the binary
CMD ["./main"]
