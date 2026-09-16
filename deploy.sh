#!/bin/bash

# Выход при любой ошибке
set -e

echo "=== Старт развертывания через Docker Compose ==="

# 1. Проверка прав суперпользователя (для генерации SSL и работы с Docker)
if [ "$EUID" -ne 0 ]; then
  echo "❌ Пожалуйста, запустите скрипт с правами sudo"
  exit 1
fi

# 2. Проверка наличия Docker и Docker Compose
if ! command -v docker &> /dev/null || ! command -v docker compose &> /dev/null; then
  echo "📦 Установка Docker и Docker Compose..."
  apt-get update
  apt-get install -y ca-certificates curl gnupg lsb-release
  
  # Добавление официального репозитория Docker
  mkdir -p /etc/apt/keyrings
  curl -fsSL https://docker.com | gpg --dearmor -o /etc/apt/keyrings/docker.gpg
  echo "deb [arch=$(dpkg --print-architecture) signed-by=/etc/apt/keyrings/docker.gpg] https://docker.com $(lsb_release -cs) stable" | tee /etc/apt/sources.list.min.d/docker.list > /dev/null
  
  apt-get update
  apt-get install -y docker-ce docker-ce-cli containerd.io docker-compose-plugin
fi

# 3. Генерация SSL-сертификатов на хосте
if [ ! -f "server.crt" ] || [ ! -f "server.key" ]; then
  echo "🔐 SSL-сертификаты не найдены. Генерация самоподписанных сертификатов..."
  openssl req -x509 -newkey rsa:4096 -keyout server.key -out server.crt -days 365 -nodes -subj "/CN=localhost"
  # Устанавливаем права на чтение для контейнера
  chmod 644 server.crt server.key
else
  echo "🔐 Использование существующих SSL-сертификатов (server.crt/server.key)"
fi

# 4. Сборка и запуск контейнера в фоне
echo "🏗 Сборка Docker-образа и запуск контейнера..."
docker compose up --build -d

echo "======================================================="
echo "✅ Деплой успешно завершен!"
echo "📺 Стример доступен по адресу: https://<IP_СЕРВЕРА>:8443"
echo "📊 Логи контейнера: docker compose logs -f"
echo "======================================================="
