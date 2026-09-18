#!/bin/bash
set -e

# === ССЫЛКИ НА ВАШ РЕПОЗИТОРИЙ GITHUB ===
# (Замените AlexanderV823/webcam-streamer на ваш актуальный репозиторий webcam-streamer)
REPO_RAW_URL="https://raw.githubusercontent.com/AlexanderV823/webcam-streamer"
SERVER_PATH="/opt/webcam-streamer"

echo "🔍 === 1. Проверка системных зависимостей на сервере ==="
command -v curl >/dev/null 2>&1 || (echo "📥 Установка curl..." && sudo apt-get update && sudo apt-get install -y curl)
command -v docker >/dev/null 2>&1 || (echo "🐳 Установка Docker..." && curl -fsSL https://docker.com | sh)
command -v openssl >/dev/null 2>&1 || (echo "🔑 Установка OpenSSL..." && sudo apt-get update && sudo apt-get install -y openssl)
command -v nano >/dev/null 2>&1 || (echo "📝 Установка nano..." && sudo apt-get update && sudo apt-get install -y nano)
echo "✅ Все системные зависимости проверены и установлены."

echo "📂 === 2. Подготовка рабочей директории ==="
sudo mkdir -p "$SERVER_PATH"
cd "$SERVER_PATH"
echo "📂 Рабочая директория готова: $SERVER_PATH"

echo "🛜 === 3. Скачивание конфигураций с GitHub ==="
sudo curl -sSLO $REPO_RAW_URL/docker-compose.yml
echo "⬇️  [1/4] docker-compose.yml загружен"
sudo curl -sSLO $REPO_RAW_URL/nginx.conf
echo "⬇️  [2/4] nginx.conf загружен"
sudo curl -sSLO $REPO_RAW_URL/.env.example
echo "⬇️  [3/4] .env.example загружен"
sudo curl -sSLO $REPO_RAW_URL/Dockerfile
echo "⬇️  [4/4] Dockerfile загружен"
echo "✨ Все файлы конфигурации успешно скачаны."

echo "⚙️ === 4. Инициализация .env, определение реального IP и JWT ==="
if [ ! -f .env ]; then
    sudo cp .env.example .env
    
    # 1. Автоматически определяем внешний IP-адрес роутера Keenetic
    REAL_IP=$(curl -s ifconfig.me || echo "127.0.0.1")
    sudo sed -i "s|^SERVER_IP=.*|SERVER_IP=$REAL_IP|" .env
    echo "🌐 Реальный IP-адрес ($REAL_IP) определен и записан в .env"
    
    # 2. Генерируем случайный 32-байтовый шестнадцатеричный ключ для JWT
    JWT_GEN=$(openssl rand -hex 32)
    sudo sed -i "s|^JWT_SECRET=.*|JWT_SECRET=$JWT_GEN|" .env
    echo "🔑 Уникальный криптографический JWT_SECRET успешно добавлен в .env"
else
    echo "ℹ️  Файл .env уже существует на сервере, пропускаем автоматическое заполнение."
fi

echo "📝 === 5. Интерактивная настройка параметров в nano ==="
echo "🔔 ВНИМАНИЕ: Проверьте SERVER_IP (или укажите, например, KeenDNS) и введите ADMIN_PASSWORD_HASH."
echo "💾 Сохранить: Ctrl+O -> Enter | ❌ Выход: Ctrl+X"
sleep 3
sudo nano .env

echo "🚀 === 6. Запуск контейнеров в Docker Compose ==="
echo "🔄 Перезапуск Docker-сервисов..."
sudo docker compose down
# --build принудительно пересоберет Go приложение из обновленного Dockerfile
sudo docker compose up -d --build

echo "🎉 === [SUCCESS] Деплой webcam-streamer успешно завершен! ==="
echo "📊 Посмотреть статус контейнеров: sudo docker compose ps"
echo "📜 Посмотреть логи трансляции:    sudo docker compose logs -f"
