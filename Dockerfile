# === Этап 1: Сборка бинарника ===
FROM golang:1.25-bookworm AS builder

# Устанавливаем рабочую директорию внутри контейнера
WORKDIR /app

# Копируем зависимости
COPY go.mod go.sum ./
RUN go mod download

# Копируем весь исходный код проекта
COPY . .

# Включаем CGO_ENABLED=1 и собираем под нативный Debian Linux (glibc)
ENV CGO_ENABLED=1
ENV GOOS=linux
ENV GOARCH=amd64
RUN go build -ldflags="-w -s" -o webcam-streamer cmd/server/main.go

# === Этап 2: Финальный продакшн-контейнер ===
# Используем Debian Slim вместо Alpine, чтобы glibc совпадал с хостом
FROM debian:bookworm-slim

# Устанавливаем сертификаты безопасности и часовые пояса
RUN apt-get update && apt-get install -y --no-install-recommends \
    ca-certificates \
    tzdata \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /app

# Копируем скомпилированное приложение
COPY --from=builder /app/webcam-streamer .

# Точка запуска приложения при старте контейнера
ENTRYPOINT ["./webcam-streamer"]
