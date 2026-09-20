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
echo "✅ Все системные зависимости проверены и установлены."

echo "🧹 === 2. Очистка старых ресурсов и подготовка директории ==="
if [ -d "$SERVER_PATH" ]; then
    echo "🔄 Обнаружена существующая директория проекта. Запуск глубокой очистки..."
    cd "$SERVER_PATH"

    # Если в папке есть старый docker-compose.yml, останавливаем запущенные контейнеры,
    # удаляем их анонимные тома (-v) и контейнеры-сироты (--remove-orphans)
    if [ -f "docker-compose.yml" ]; then
        echo "🛑 Остановка и удаление старых контейнеров проекта..."
        sudo docker compose down -v --remove-orphans >/dev/null 2>&1 || true
    fi

    # Удаляем старые конфигурационные файлы, чтобы скачать свежие.
    # Файл .env тоже удаляем, так как это чистая установка с нуля.
    echo "🗑️  Удаление старых конфигурационных файлов..."
    sudo rm -f docker-compose.yml nginx.conf .env.example .env
else
    echo "📂 Создание новой рабочей директории..."
    sudo mkdir -p "$SERVER_PATH"
fi

# Делаем текущего пользователя владельцем папки
sudo chown -R $USER:$USER "$SERVER_PATH"
cd "$SERVER_PATH"
echo "✅ Рабочая директория полностью очищена и готова: $SERVER_PATH"

echo "🛜 === 3. Скачивание конфигураций с GitHub ==="
# Скачиваем ТОЛЬКО конфигурации оркестрации и прокси. Исходный код скачает сам Docker.
sudo curl -sSLO "$REPO_RAW_URL/docker-compose.yml"
echo "⬇️  [1/3] docker-compose.yml загружен"
sudo curl -sSLO "$REPO_RAW_URL/nginx.conf"
echo "⬇️  [2/3] nginx.conf загружен"
sudo curl -sSLO "$REPO_RAW_URL/.env.example"
echo "⬇️  [3/3] .env.example загружен"
echo "✨ Все необходимые конфигурации успешно развернуты на сервере."

echo "⚙️ === 4. Подготовка шаблона конфигурации ==="
if [ ! -f .env ]; then
    # Создаем чистый понятный шаблон для пользователя, копируя пример
    cp .env.example .env
    # На всякий случай гарантируем наличие базовых строк для заполнения
    if ! grep -q "ADMIN_PASSWORD=" .env; then
        echo -e "\nADMIN_PASSWORD=" >> .env
    fi
else
    echo "ℹ️  Файл .env уже существует на сервере."
fi

echo "📝 === 5. Интерактивная настройка параметров пользователем ==="
echo "--------------------------------------------------------------------------------"
echo "💡 ИНСТРУКЦИЯ ПО НАСТРОЙКЕ:"
echo "1️⃣  Укажите ваш пароль в строке: ADMIN_PASSWORD=ваш_пароль (простым текстом)"
echo "2️⃣  Проверьте WEB_PORT (по умолчанию 80, измените если занят, например на 8085)"
echo "3️⃣  SERVER_IP оставьте AUTODETECT или впишите ваш Keenetic домен вручную"
echo "--------------------------------------------------------------------------------"
echo "💾 Сохранить изменения: Ctrl+O -> Нажать Enter"
echo "❌ Выйти из редактора:   Ctrl+X"
echo "--------------------------------------------------------------------------------"
echo "⏳ Запуск редактора через 3 секунды..."
sleep 3
nano .env

echo "🔒 === 6. Автоматическая постобработка и генерация секретов ==="
# Извлекаем чистый пароль, введенный пользователем
ADMIN_PASS=$(grep -E "^ADMIN_PASSWORD=" .env | cut -d'=' -f2-)

if [ -z "$ADMIN_PASS" ]; then
    echo "❌ Ошибка: Вы оставили поле ADMIN_PASSWORD пустым! Деплой остановлен."
    exit 1
fi

# 1. Заменяем SERVER_IP=AUTODETECT на реальный IP, если пользователь оставил автоопределение
TEMPLATE_IP=$(grep -E "^SERVER_IP=" .env | cut -d'=' -f2-)
if [ "$TEMPLATE_IP" = "AUTODETECT" ]; then
    REAL_IP=$(curl -s ifconfig.me || echo "127.0.0.1")
    sudo sed -i "s|^SERVER_IP=.*|SERVER_IP=$REAL_IP|" .env
    echo "🌐 Реальный IP-адрес ($REAL_IP) определен и записан."
fi

# 2. Генерируем JWT_SECRET, если его еще нет в файле
if ! grep -q "^JWT_SECRET=" .env; then
    JWT_GEN=$(openssl rand -hex 32)
    echo "JWT_SECRET=$JWT_GEN" >> .env
    echo "🔑 Уникальный криптографический JWT_SECRET успешно сгенерирован и добавлен."
fi

# 3. Генерируем НАСТОЯЩИЙ Bcrypt-хэш пароля
echo "⏳ Генерация криптографического Bcrypt-хэша пароля..."
RAW_HASH=$(docker run --rm alpine:3.19 sh -c "apk add --no-cache apache2-utils >/dev/null && htpasswd -B -n -b admin '$ADMIN_PASS'" | sed 's/^admin://' | tr -d '\r\n')

if [ -z "$RAW_HASH" ]; then
    echo "❌ Ошибка: Не удалось сгенерировать Bcrypt-хэш пароля."
    exit 1
fi

BCRYPT_HASH="${RAW_HASH//\$/\$\$}"

# Полностью очищаем текстовый пароль из файла для безопасности
sudo sed -i '/^ADMIN_PASSWORD=/d' .env

# Дописываем экранированный хэш в .env
echo "ADMIN_PASSWORD_HASH=$BCRYPT_HASH" >> .env

unset ADMIN_PASS
echo "✅ Текстовый пароль успешно удален и заменен на безопасный Bcrypt-хэш в конце .env."

echo "🚀 === 7. Запуск контейнеров в Docker Compose ==="
echo "🔄 Сборка и запуск Docker-сервисов..."

sudo docker compose build --pull || sudo docker-compose build --pull

# Запускаем контейнеры в фоновом режиме
sudo docker compose up -d || sudo docker-compose up -d

echo "🧹 === 8. Очистка устаревших Docker-ресурсов ==="
echo "⏳ Удаление неиспользуемых образов-сирот (<none>)..."
# docker image prune -f удаляет ТОЛЬКО промежуточные слои и старые образы без тегов,
# оставшиеся от предыдущих сборок. Ваши рабочие контейнеры и образы Nginx/Go не пострадают!
sudo docker image prune -f

# Выводим текущее состояние диска
echo "💾 Текущий баланс дискового пространства на сервере:"
df -h / | awk 'NR==2 {print "   Доступно: " $4 " из " $2 " (Использовано: " $5 ")"}'

cd "$SERVER_PATH"

echo "🎉 === [SUCCESS] Деплой webcam-streamer успешно завершен! ==="
echo "📊 Посмотреть статус контейнеров: sudo docker compose ps"
echo "📜 Посмотреть логи трансляции:    sudo docker compose logs -f"
