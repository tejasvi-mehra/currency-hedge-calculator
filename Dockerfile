FROM golang:1.24-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 go build -o /bin/currency-hedge-calculator .

FROM alpine:3.21

RUN adduser -D -g '' appuser
WORKDIR /app

COPY --from=builder /bin/currency-hedge-calculator /usr/local/bin/currency-hedge-calculator
COPY data ./data
COPY docs ./docs
COPY .env.example ./.env.example

ENV APP_ENV=production
ENV LOG_ENCODING=json
ENV SERVER_LISTEN_ADDR=:8080

EXPOSE 8080

USER appuser
ENTRYPOINT ["currency-hedge-calculator"]
