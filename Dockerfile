# Stage 1: Build
FROM golang:1.26-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /proxy ./cmd/proxy

# Stage 2: Runtime
FROM alpine:3.21
RUN apk add --no-cache ca-certificates
COPY --from=builder /proxy /proxy
COPY configs/config.example.yaml /etc/midproxy/config.yaml
EXPOSE 8080
ENTRYPOINT ["/proxy", "-config", "/etc/midproxy/config.yaml"]
