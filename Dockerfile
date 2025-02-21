# Build stage
FROM golang:1.22-alpine AS builder

# Install build dependencies
RUN apk add --no-cache python3 py3-pip gcc musl-dev

# Set working directory
WORKDIR /app

# Copy go mod files
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Install yt-dlp with explicit pip commands
RUN python3 -m pip install --upgrade pip --break-system-packages && \
    python3 -m pip install --upgrade yt-dlp --break-system-packages

# Build the application
RUN go build -o bin/app cmd/transcription/main.go

# Final stage
FROM alpine:latest

# Install runtime dependencies including ffmpeg
RUN apk add --no-cache python3 py3-pip ffmpeg

# Install yt-dlp with explicit pip commands
RUN python3 -m pip install --upgrade pip --break-system-packages && \
    python3 -m pip install --upgrade yt-dlp --break-system-packages

# Create temp directory with proper permissions
RUN mkdir -p /app/temp && chmod 777 /app/temp

# Copy the binary from builder
COPY --from=builder /app/bin/app /app/bin/app

# Set the working directory
WORKDIR /app

# Command to run
CMD ["/app/bin/app", "TODO"]