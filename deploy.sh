#!/bin/bash
set -e

# === ССЫЛКИ НА ВАШ РЕПОЗИТОРИЙ GITHUB ===
# (Замените AlexanderV823/webcam-streamer на ваш актуальный репозиторий webcam-streamer)
REPO_RAW_URL="https://raw.githubusercontent.com/AlexanderV823/webcam-streamer/main"
SERVER_PATH="/opt/webcam-streamer"

echo "🔍 === 1. Проверка системных зависимостей на сервере ==="
command -v curl >/dev/null 2>&1 || (echo "📥 Установка curl..." && sudo apt-get update && sudo apt-get install -y curl)
command -v docker >/dev/null 2>&1 || (echo "🐳 Установка Docker..." && curl -fsSL https://get.docker.com | sh)
command -v openssl >/dev/null 2>&1 || (echo "🔑 Установка OpenSSL..." && sudo apt-get update && sudo apt-get install -y openssl)
command -v nano >/dev/null 2>&1 || (echo "📝 Установка nano..." && sudo apt-get update && sudo apt-get install -y nano)
command -v tar >/dev/null 2>&1 || (echo "📦 Установка tar..." && sudo apt-get update && sudo apt-get install -y tar)
echo "✅ Все системные зависимости проверены и установлены."

echo "📂 === 2. Подготовка рабочей директории ==="
sudo mkdir -p "$SERVER_PATH"
cd "$SERVER_PATH"
echo "📂 Рабочая директория готова: $SERVER_PATH"

echo "🛜 === 3. Скачивание конфигураций с GitHub ==="
# Скачиваем ТОЛЬКО конфигурации оркестрации и прокси. Исходный код скачает сам Docker.
sudo curl -sSLO "$REPO_RAW_URL/docker-compose.yml"
echo "⬇️  [1/3] docker-compose.yml загружен"
sudo curl -sSLO "$REPO_RAW_URL/nginx.conf"
echo "⬇️  [2/3] nginx.conf загружен"
sudo curl -sSLO "$REPO_RAW_URL/.env.example"
echo "⬇️  [3/3] .env.example загружен"
echo "✨ Все необходимые конфигурации успешно развернуты на сервере."

echo "⚙️ === 4. Инициализация .env, определение реального IP и JWT ==="
if [ ! -f .env ]; then
    sudo cp .env.example .env

    # Проверяем, стоит ли флаг автоопределения IP в шаблоне
    TEMPLATE_IP=$(grep -E "^SERVER_IP=" .env | cut -d'=' -f2-)

    if [ "$TEMPLATE_IP" = "AUTODETECT" ]; then
        # Автоматически определяем внешний IP-адрес
        REAL_IP=$(curl -s ifconfig.me || echo "127.0.0.1")
        sudo sed -i "s|^SERVER_IP=.*|SERVER_IP=$REAL_IP|" .env
        echo "🌐 Реальный IP-адрес ($REAL_IP) определен и записан в .env"
    else
        echo "💻 В .env.example задан фиксированный IP/домен ($TEMPLATE_IP). Автоопределение пропущено."
    fi

    # Генерируем случайный 32-байтовый шестнадцатеричный ключ для JWT
    JWT_GEN=$(openssl rand -hex 32)
    sudo sed -i "s|^JWT_SECRET=.*|JWT_SECRET=$JWT_GEN|" .env
    echo "🔑 Уникальный криптографический JWT_SECRET успешно добавлен в .env"
else
    echo "ℹ️  Файл .env уже существует на сервере, пропускаем автоматическое заполнение."
fi

echo "📝 === 5. Интерактивная настройка параметров в nano ==="
echo "--------------------------------------------------------------------------------"
echo "💡 ПОДСКАЗКА ПО НАСТРОЙКЕ КАНАЛОВ И ОТЛАДКИ:"
echo "1️⃣  Для локальной отладки: сотрите автоматически подставленный IP"
echo "    в первой строчке и напишите вручную: SERVER_IP=localhost (или 127.0.0.1)."
echo "2️⃣  Для работы за роутером: оставьте внешний IP или укажите ваш домен."
echo "3️⃣  Обязательно введите ваш пароль в поле ADMIN_PASSWORD простым текстом."
echo "--------------------------------------------------------------------------------"
echo "💾 Сохранить изменения: Ctrl+O -> Нажать Enter"
echo "❌ Выйти из редактора:   Ctrl+X"
echo "--------------------------------------------------------------------------------"
echo "⏳ Запуск редактора через 5 секунд..."
sleep 5
sudo nano .env

echo "🔒 === 6. Перехват пароля и автоматическая генерация Bcrypt-хэша ==="
ADMIN_PASS=$(grep -E "^ADMIN_PASSWORD=" .env | cut -d'=' -f2-)

if [ -z "$ADMIN_PASS" ]; then
    echo "❌ Ошибка: Вы оставили поле ADMIN_PASSWORD пустым! Деплой остановлен."
    exit 1
fi

echo "⏳ Генерация криптографического хэша пароля..."
# Используем легковесный образ alpine с утилитой htpasswd
BCRYPT_HASH=$(docker run --rm alpine:3.19 sh -c "apk add --no-cache apache2-utils >/dev/null && htpasswd -B -n -b admin '$ADMIN_PASS'" | cut -d':' -f2)

# Заменяем текстовый пароль на безопасный Bcrypt-хэш
sudo sed -i "s|^ADMIN_PASSWORD=.*|ADMIN_PASSWORD_HASH=$BCRYPT_HASH|" .env
unset ADMIN_PASS
echo "✅ Текстовый пароль успешно заменен на безопасный Bcrypt-хэш."

echo "🚀 === 7. Запуск контейнеров в Docker Compose ==="
echo "🔄 Перезапуск Docker-сервисов..."
sudo docker compose down
# Docker автоматически скачает актуальный код ветки main с GitHub и соберет его
sudo docker compose up -d --build

echo "🧹 === 8. Очистка устаревших Docker-ресурсов ==="
echo "⏳ Удаление неиспользуемых образов-сирот (<none>)..."
# docker image prune -f удаляет ТОЛЬКО промежуточные слои и старые образы без тегов,
# оставшиеся от предыдущих сборок. Ваши рабочие контейнеры и образы Nginx/Go не пострадают!
sudo docker image prune -f

# Выводим текущее состояние диска
echo "💾 Текущий баланс дискового пространства на сервере:"
df -h / | awk 'NR==2 {print "   Доступно: " $4 " из " $2 " (Использовано: " $5 ")"}'

echo "🎉 === [SUCCESS] Деплой webcam-streamer успешно завершен! ==="
echo "📊 Посмотреть статус контейнеров: sudo docker compose ps"
echo "📜 Посмотреть логи трансляции:    sudo docker compose logs -f"
