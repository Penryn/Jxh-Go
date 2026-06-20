FROM golang:1.25-trixie AS build

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /out/jxh-bot ./cmd/bot

FROM debian:trixie-slim

WORKDIR /app
RUN apt-get update \
    && apt-get install -y --no-install-recommends ca-certificates tzdata \
    && rm -rf /var/lib/apt/lists/* \
    && useradd --create-home --home-dir /app --shell /usr/sbin/nologin appuser
COPY --from=build /out/jxh-bot /usr/local/bin/jxh-bot
COPY config.example.yaml /app/config.yaml
RUN mkdir -p /app/data/cache && chown -R appuser:appuser /app
USER appuser

EXPOSE 8080
ENTRYPOINT ["jxh-bot", "-config", "/app/config.yaml"]
