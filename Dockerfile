# Build stage
FROM golang:1.25-alpine AS builder

# Install git for dependency management and ca-certificates for HTTPS requests
RUN apk add --no-cache git ca-certificates

# Set working directory
WORKDIR /app

# Copy local dependencies first
# COPY SkyCrypt-Types/ ../SkyCrypt-Types/
# COPY SkyHelper-Networth-Go/ ../SkyHelper-Networth-Go/

# Copy go mod files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Build the application
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -buildvcs=false -o main .

# Final stage
FROM alpine:latest

# Install ca-certificates for HTTPS requests
RUN apk --no-cache add ca-certificates git

# Create app directory
WORKDIR /root/

# Copy the binary from builder stage
COPY --from=builder /app/main .

# Copy assets and other necessary files
COPY --from=builder /app/assets ./assets
COPY --from=builder /app/NotEnoughUpdates-REPO ./NotEnoughUpdates-REPO
COPY --from=builder /app/docs ./docs

# Expose port
EXPOSE 8080

# Command to run the application
CMD ["./main"]
