#!/bin/bash

# Выход при любой ошибке
set -e

echo "=== Деплой Go Streamer + Nginx + Basic Auth ==="

# 1. Проверка прав суперпользователя (для генерации SSL и работы с Docker)
if [ "$EUID" -ne 0 ]; then
  echo "❌ Пожалуйста, запустите скрипт с правами sudo"
  exit 1
fi

# 1. Создание и интерактивное редактирование .env
if [ ! -f ".env" ]; then
  echo "📝 Файл .env не найден. Создаю его из примера..."
  cp .env.example .env
  
  echo "⚙️ Открываю текстовый редактор nano..."
  echo "👉 Задайте значения для DOMAIN_NAME, AUTH_USER и AUTH_PASSWORD."
  echo "👉 Для сохранения нажмите Ctrl+O, затем Enter. Для выхода: Ctrl+X."
  sleep 3 # Небольшая пауза, чтобы пользователь успел прочитать подсказку
  
  # Открываем nano напрямую в текущей сессии терминала
  nano .env
  
  echo "✅ Файл .env сохранен. Продолжаю процесс развертывания..."
fi

# Загружаем переменные из .env
export $(grep -v '^#' .env | xargs)

if [ "$DOMAIN_NAME" == "localhost" ] || [ -z "$DOMAIN_NAME" ]; then
  echo "❌ Ошибка: В .env указан домен '$DOMAIN_NAME'. Для Let's Encrypt нужен реальный домен!"
  exit 1
fi

if [ -z "$AUTH_USER" ] || [ -z "$AUTH_PASSWORD" ]; then
  echo "❌ Ошибка: AUTH_USER или AUTH_PASSWORD не могут быть пустыми в .env!"
  exit 1
fi

# 2. Генерация файла .htpasswd средствами openssl
echo "🔐 Генерация файла паролей .htpasswd для пользователя: $AUTH_USER..."
# Форматируем пароль по стандарту веб-серверов с использованием алгоритма crypt
BCRYPT_PASSWORD=$(openssl passwd -crypt "$AUTH_PASSWORD")
echo "${AUTH_USER}:${BCRYPT_PASSWORD}" > .htpasswd
chmod 644 .htpasswd

# 3. Подстановка домена в nginx.conf
echo "⚙️ Настройка конфигурации Nginx под домен $DOMAIN_NAME..."
sed -i "s/server_name .*/server_name $DOMAIN_NAME;/g" nginx.conf
sed -i "s|/live/[^/]*/|/live/$DOMAIN_NAME/|g" nginx.conf

# 4. Первый запуск Nginx для прохождения проверки Certbot
echo "🏗 Запуск временного Nginx для выпуска сертификата..."
docker compose up -d nginx

# 5. Запрос сертификата Let's Encrypt
if [ ! -d "./certbot/conf/live/$DOMAIN_NAME" ]; then
  echo "🔐 Запрос SSL сертификата у Let's Encrypt для $DOMAIN_NAME..."
  docker compose run --rm certbot certonly --webroot -w /var/www/certbot \
    -d "$DOMAIN_NAME" --email "$CERTBOT_EMAIL" --rsa-key-size 4096 \
    --agree-tos --non-interactive
else
  echo "🔐 Сертификаты Let's Encrypt уже существуют."
fi

# 6. Полный перезапуск всей инфраструктуры
echo "🚀 Перезапуск всех сервисов в боевом режиме..."
docker compose down
docker compose up --build -d

echo "======================================================="
echo "✅ Инфраструктура успешно развернута!"
echo "📺 Стрим защищен Basic Auth и доступен по адресу: https://$DOMAIN_NAME"
echo "👤 Логин: $AUTH_USER"
echo "======================================================="
