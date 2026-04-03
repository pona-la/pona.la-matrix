# Build stage
FROM golang:1.21-alpine AS builder

WORKDIR /app

# Copy go.mod file
COPY go.mod ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Build the application
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o lldap-reg .

# Runtime stage
FROM alpine:latest

RUN apk --no-cache add ca-certificates

WORKDIR /root/

# Copy the binary from builder stage
COPY --from=builder /app/lldap-reg .

# Run the application
CMD ["./lldap-reg"]
