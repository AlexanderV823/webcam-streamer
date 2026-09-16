# --- Этап 1: Сборка бинарника ---
FROM golang:1.23-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
# Собираем статический бинарник без CGO
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o streamer main.go

# --- Этап 2: Финальный легковесный образ ---
FROM alpine:3.20
RUN apk add --no-cache tzdata openssl
WORKDIR /app

# Копируем бинарник из первого этапа
COPY --from=builder /app/streamer .

# Экспонируем HTTPS порт
EXPOSE 8443

# Запуск приложения
CMD ["./streamer"]
