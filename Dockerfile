# === Этап 1: Сборка бинарника ===
FROM golang:1.25-alpine AS builder

# Устанавливаем рабочую директорию внутри контейнера
WORKDIR /app

# Копируем файлы описания зависимостей
COPY go.mod go.sum ./

# Скачиваем зафиксированные внешние пакеты (x/sys, x/time, x/crypto)
RUN go mod download

# Копируем весь исходный код проекта webcam-streamer
COPY . .

# Компилируем оптимизированный статичный бинарник Go без CGO флага
# Флаги -w -s очищают отладочную информацию и уменьшают размер файла
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-w -s" -o webcam-streamer cmd/server/main.go

# === Этап 2: Финальный продакшн-контейнер ===
FROM alpine:3.19

# Устанавливаем сертификаты безопасности и данные часовых поясов
RUN apk --no-cache add ca-certificates tzdata

WORKDIR /app

# Копируем скомпилированное приложение из предыдущего этапа
COPY --from=builder /app/webcam-streamer .

# Точка запуска приложения при старте контейнера
ENTRYPOINT ["./webcam-streamer"]
