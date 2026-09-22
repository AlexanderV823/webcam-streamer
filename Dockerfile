# === Этап 1: Сборка бинарника ===
FROM golang:1.25-bookworm AS builder

# Устанавливаем рабочую директорию внутри контейнера
WORKDIR /app

# Шаг 1. Кэшируем зависимости (скачиваются заново только при изменении go.mod)
COPY go.mod go.sum ./
RUN go mod download

# Шаг 2. Копируем код и собираем (этот слой Docker всегда пересоберет, если изменился коммит)
COPY . .
ENV CGO_ENABLED=1 GOOS=linux GOARCH=amd64
RUN go build -ldflags="-w -s" -o webcam-streamer cmd/server/main.go


# === Этап 2: Финальный продакшн-контейнер ===
# Используем Debian Slim вместо Alpine, чтобы glibc совпадал с хостом
FROM debian:bookworm-slim

# Шаг 3. Установка системных утилит.
# Этот слой выполнится ровно ОДИН РАЗ при первой сборке и закэшируется Docker.
RUN apt-get update && apt-get install -y --no-install-recommends \
    ca-certificates \
    tzdata \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /app

# Шаг 4. Копируем бинарник (этот слой обновляется всегда)
COPY --from=builder /app/webcam-streamer .

# Точка запуска приложения при старте контейнера
ENTRYPOINT ["./webcam-streamer"]
