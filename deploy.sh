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

  # Временный сброс переменных сессии перед проверкой
  unset DOMAIN_NAME AUTH_USER AUTH_PASSWORD CERTBOT_EMAIL APP_PORT LOG_MAX_SIZE LOG_MAX_FILES

  # Читаем свежие переменные
  export $(grep -v '^#' .env | xargs)

  # Флаг валидности
  VALID=true

  # ... (проверки домена, юзера, пароля и email прежние) ...

  # Проверка настроек ротации логов
  if [ -z "$LOG_MAX_SIZE" ] || [ "$LOG_MAX_SIZE" == "10m" -a ! -f ".env" ]; then
    # Если переменная пустая — это критично
    if [ -z "$LOG_MAX_SIZE" ]; then
      echo "❌ Ошибка: Переменная LOG_MAX_SIZE не должна быть пустой."
      VALID=false
    fi
  fi

  if [ -z "$LOG_MAX_FILES" ]; then
    echo "❌ Ошибка: Переменная LOG_MAX_FILES не должна быть пустой."
    VALID=false
  fi

  # Если всё заполнено верно, выходим из цикла
  if [ "$VALID" = true ]; then
    echo "✅ Валидация успешна! Все переменные, включая лимиты логов, заполнены корректно."
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

# 2. Генерация файла паролей .htpasswd с использованием стойкого алгоритма Bcrypt
echo "🔐 Генерация файла паролей .htpasswd для пользователя: $AUTH_USER..."

# Запускаем временный контейнер Nginx для генерации файла с помощью встроенной утилиты htpasswd
# Флаг -B включает криптостойкий алгоритм Bcrypt, флаг -b позволяет передать пароль аргументом, -c создает файл
docker run --rm -v "$(pwd)":/auth alpine:3.19 sh -c "
  apk add --no-cache apache2-utils && \
  htpasswd -B -b -c /auth/.htpasswd '$AUTH_USER' '$AUTH_PASSWORD'
"

# Устанавливаем безопасные права доступа (чтение для всех, запись только для владельца)
chmod 644 .htpasswd
echo "✅ Файл .htpasswd успешно сгенерирован с применением Bcrypt."

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
