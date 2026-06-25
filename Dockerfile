FROM golang:1.25-alpine AS builder

WORKDIR /app
COPY . .

RUN go mod download && \
    go build -o notification-server ./cmd/notification-server

FROM alpine:latest

RUN apk --no-cache add ca-certificates

WORKDIR /root/
COPY --from=builder /app/notification-server .

RUN mkdir -p /data

EXPOSE 8080

CMD ["./notification-server"]
