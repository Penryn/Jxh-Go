FROM golang:1.25-alpine AS build

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /out/jxh-bot ./cmd/bot

FROM alpine:3.22

WORKDIR /app
RUN adduser -D -H appuser
COPY --from=build /out/jxh-bot /usr/local/bin/jxh-bot
COPY config.example.yaml /app/config.yaml
RUN mkdir -p /app/data/cache && chown -R appuser:appuser /app
USER appuser

EXPOSE 8080
ENTRYPOINT ["jxh-bot", "-config", "/app/config.yaml"]
