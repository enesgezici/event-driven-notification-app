FROM golang:1.25-alpine AS builder

WORKDIR /app
COPY source/go.mod source/go.sum ./

RUN go mod download

COPY source/ ./

RUN go build -o notification-server ./cmd/notification-server

FROM alpine:latest

RUN apk --no-cache add ca-certificates curl

WORKDIR /root/
COPY --from=builder /app/notification-server .

EXPOSE 8080

CMD ["./notification-server"]
