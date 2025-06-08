FROM golang:1.23-alpine3.21 AS builder

WORKDIR /app

COPY . /app

RUN go mod download

RUN CGO_ENABLED=0 GOOS=linux go build -ldflags "-s -w" -o server exe/server/server.go

FROM alpine:3.21

WORKDIR /app

COPY --from=builder /app/server /usr/local/bin

CMD ["/usr/local/bin/server"]