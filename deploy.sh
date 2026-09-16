#!/bin/bash

# Выход при любой ошибке
set -e

echo "=== Деплой Go Streamer + Nginx + Basic Auth ==="

# 1. Проверка прав суперпользователя (для генерации SSL и работы с Docker)
if [ "$EUID" -ne 0 ]; then
  echo "❌ Пожалуйста, запустите скрипт с правами sudo"
  exit 1
fi

# 1. Создание, интерактивное редактирование и валидация .env
while true; do
  if [ ! -f ".env" ]; then
    echo "📝 Файл .env не найден. Создаю его из примера..."
    cp .env.example .env
    
    echo "⚙️ Открываю текстовый редактор nano..."
    echo "👉 Задайте значения для DOMAIN_NAME, AUTH_USER и AUTH_PASSWORD."
    echo "👉 Для сохранения нажмите Ctrl+O, затем Enter. Для выхода: Ctrl+X."
    sleep 3
    
    nano .env
  fi

  echo "🔍 Проверяю корректность заполнения .env..."
  
  # Временный сброс переменных, чтобы старые данные из сессии не мешали проверке
  unset DOMAIN_NAME AUTH_USER AUTH_PASSWORD CERTBOT_EMAIL APP_PORT
  
  # Читаем свежие переменные из файла
  export $(grep -v '^#' .env | xargs)

  # Флаг валидности
  VALID=true

  # Проверка домена
  if [ -z "$DOMAIN_NAME" ] || [ "$DOMAIN_NAME" == "localhost" ] || [ "$DOMAIN_NAME" == "example.com" ]; then
    echo "❌ Ошибка: Переменная DOMAIN_NAME пустая или содержит некорректный домен ($DOMAIN_NAME)."
    VALID=false
  fi

  # Проверка логина
  if [ -z "$AUTH_USER" ] || [ "$AUTH_USER" == "admin" ]; then
    echo "❌ Ошибка: AUTH_USER пустой или оставлен дефолтным (admin)."
    VALID=false
  fi

  # Проверка пароля
  if [ -z "$AUTH_PASSWORD" ] || [ "$AUTH_PASSWORD" == "my_secure_password_123" ]; then
    echo "❌ Ошибка: AUTH_PASSWORD пустой или оставлен дефолтным из примера."
    VALID=false
  fi

  # Проверка email
  if [ -z "$CERTBOT_EMAIL" ] || [ "$CERTBOT_EMAIL" == "admin@example.com" ]; then
    echo "❌ Ошибка: CERTBOT_EMAIL пустой или оставлен дефолтным."
    VALID=false
  fi

  # Если всё заполнено верно, выходим из цикла и идем дальше по скрипту
  if [ "$VALID" = true ]; then
    echo "✅ Валидация успешна! Переменные окружения заполнены корректно."
    break
  else
    echo "⚠️ Конфигурация содержит ошибки. Перезапустить редактирование .env? (y/n)"
    read -r response
    if [[ "$response" =~ ^([yY][eE][sS]|[yY])$ ]]; then
      # Удаляем некорректный .env, чтобы цикл создал его заново на следующем круге
      rm .env
    else
      echo "🛑 Развертывание прервано пользователем."
      exit 1
    fi
  fi
done

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
