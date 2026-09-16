#!/bin/bash

# Выход при любой ошибке
set -e

echo "=== Старт развертывания Go WebCam Streamer ==="

# 1. Проверка прав суперпользователя
if [ "$EUID" -ne 0 ]; then
  echo "❌ Пожалуйста, запустите скрипт с правами sudo"
  exit 1
fi

# 2. Установка системных зависимостей Debian
echo "📦 Установка системных пакетов..."
apt update && apt install -y v4l-utils openssl golang

# 3. Подготовка директории приложения
TARGET_DIR="/usr/local/bin/webcam-streamer"
echo "📂 Подготовка директории $TARGET_DIR..."
mkdir -p "$TARGET_DIR"

# 4. Генерация SSL сертификатов (если их еще нет)
if [ ! -f "server.crt" ] || [ ! -f "server.key" ]; then
  echo "🔐 Генерация самоподписанных SSL-сертификатов..."
  openssl req -x509 -newkey rsa:4096 -keyout server.key -out server.crt -days 365 -nodes -subj "/CN=localhost"
fi

# 5. Сборка Go приложения
echo "🐹 Сборка Go приложения..."
go mod tidy
go build -ldflags="-s -w" -o streamer main.go

# 6. Копирование файлов в целевую директорию
echo "🚚 Копирование файлов..."
cp streamer server.crt server.key "$TARGET_DIR/"

# 7. Настройка Systemd службы
echo "⚙️ Настройка Systemd юнита..."
cp webcam-streamer.service /etc/systemd/system/
systemctl daemon-reload

# 8. Активация и запуск сервиса
echo "🔄 Запуск службы webcam-streamer..."
systemctl enable webcam-streamer
systemctl restart webcam-streamer

echo "======================================================="
echo "✅ Деплой успешно завершен!"
echo "📺 Сервер доступен по адресу: https://localhost:8443"
echo "📊 Статус службы: systemctl status webcam-streamer"
echo "======================================================="
