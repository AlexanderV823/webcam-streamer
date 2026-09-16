#!/bin/bash

# Выход при любой ошибке
set -e

echo "=== Деплой Go Streamer + Nginx + Let's Encrypt ==="

# 1. Проверка прав суперпользователя (для генерации SSL и работы с Docker)
if [ "$EUID" -ne 0 ]; then
  echo "❌ Пожалуйста, запустите скрипт с правами sudo"
  exit 1
fi

# 1. Создание .env из примера
if [ ! -f ".env" ]; then
  echo "📝 Создание файла .env..."
  cp .env.example .env
  echo "⚠️ Внимание: отредактируйте файл .env, указав свой реальный DOMAIN_NAME, затем запустите скрипт снова."
  exit 0
fi

# Загружаем переменные из .env
export $(grep -v '^#' .env | xargs)

if [ "$DOMAIN_NAME" == "localhost" ] || [ -z "$DOMAIN_NAME" ]; then
  echo "❌ Ошибка: В .env указан домен '$DOMAIN_NAME'. Для Let's Encrypt нужен реальный домен!"
  exit 1
fi

# 2. Подстановка домена в nginx.conf
echo "⚙️ Настройка конфигурации Nginx под домен $DOMAIN_NAME..."
sed -i "s/server_name .*/server_name $DOMAIN_NAME;/g" nginx.conf
sed -i "s|/live/[^/]*/|/live/$DOMAIN_NAME/|g" nginx.conf

# 3. Первый запуск Nginx для прохождения проверки Certbot
echo "🏗 Запуск временного Nginx для выпуска сертификата..."
docker compose up -d nginx

# 4. Запрос сертификата Let's Encrypt
if [ ! -d "./certbot/conf/live/$DOMAIN_NAME" ]; then
  echo "🔐 Запрос SSL сертификата у Let's Encrypt для $DOMAIN_NAME..."
  docker compose run --rm certbot certonly --webroot -w /var/www/certbot \
    -d "$DOMAIN_NAME" --email "$CERTBOT_EMAIL" --rsa-key-size 4096 \
    --agree-tos --non-interactive
else
  echo "🔐 Сертификаты Let's Encrypt уже существуют."
fi

# 5. Полный перезапуск всей инфраструктуры
echo "🚀 Перезапуск всех сервисов в боевом режиме..."
docker compose down
docker compose up --build -d

echo "======================================================="
echo "✅ Инфраструктура успешно развернута!"
echo "📺 Защищенный стрим доступен по адресу: https://$DOMAIN_NAME"
echo "======================================================="
