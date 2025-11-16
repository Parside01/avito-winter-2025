FROM golang:1.24-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

RUN go install github.com/pressly/goose/v3/cmd/goose@latest

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -o /app/main ./cmd

FROM alpine:latest

WORKDIR /app

COPY --from=builder /app/main .
COPY --from=builder /go/bin/goose /usr/local/bin/
COPY ./migrations ./migrations

RUN apk --update --no-cache add curl

EXPOSE 8080

COPY apprun.sh .
RUN chmod +x apprun.sh

CMD ["./apprun.sh"]
