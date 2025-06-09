FROM golang:1.23-alpine3.21 AS builder

RUN apk update && apk add --no-cache build-base

WORKDIR /app

COPY . /app

RUN CGO_ENABLED=1 GOOS=linux go mod download && go build -ldflags "-s -w" -o server exe/server/server.go

FROM alpine:3.21

WORKDIR /app

COPY --from=builder /app/server /usr/local/bin

CMD ["/usr/local/bin/server"]