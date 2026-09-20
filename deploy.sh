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

echo "⚙️ === 4. Инициализация .env, определение реального IP и JWT ==="
if [ ! -f .env ]; then
    # Создаем файл .env через sudo
    sudo touch .env

    # 1. Задаем базовые параметры, используя sudo tee -a для обхода ограничений прав
    echo "ADMIN_USERNAME=admin" | sudo tee -a .env > /dev/null
    echo "DEFAULT_CAMERA=/dev/video0" | sudo tee -a .env > /dev/null
    echo "WEB_PORT=80" | sudo tee -a .env > /dev/null
    echo "🚪 Внешний порт по умолчанию (WEB_PORT=80) добавлен в .env"

    # 2. Автоматически определяем внешний IP-адрес
    REAL_IP=$(curl -s ifconfig.me || echo "127.0.0.1")
    echo "SERVER_IP=$REAL_IP" | sudo tee -a .env > /dev/null
    echo "🌐 Реальный IP-адрес ($REAL_IP) определен и записан в .env"

    # 3. Генерируем случайный 32-байтовый шестнадцатеричный ключ для JWT
    JWT_GEN=$(openssl rand -hex 32)
    echo "JWT_SECRET=$JWT_GEN" | sudo tee -a .env > /dev/null
    echo "🔑 Уникальный криптографический JWT_SECRET успешно добавлен в .env"
else
    echo "ℹ️  Файл .env уже существует на сервере, пропускаем автоматическое заполнение."
fi

echo "📝 === 5. Интерактивная настройка параметров в nano ==="
echo "--------------------------------------------------------------------------------"
echo "💡 ПОДСКАЗКА ПО НАСТРОЙКЕ КАНАЛОВ И ОТЛАДКИ:"
echo "1️⃣  Если порт 80 занят, измените строчку WEB_PORT=80 на свободный (например, 8085)."
echo "2️⃣  Для локальной отладки: сотрите автоматически подставленный IP"
echo "    в строчке SERVER_IP=... и напишите вручную: SERVER_IP=localhost"
echo "3️⃣  В самом конце файла ОБЯЗАТЕЛЬНО добавьте строчку: ADMIN_PASSWORD=ваш_пароль"
echo "--------------------------------------------------------------------------------"
echo "💾 Сохранить изменения: Ctrl+O -> Нажать Enter"
echo "❌ Выйти из редактора:   Ctrl+X"
echo "--------------------------------------------------------------------------------"
echo "⏳ Запуск редактора через 5 секунд..."
sleep 5
sudo nano .env

echo "🔒 === 6. Перехват пароля и автоматическая генерация Bcrypt-хэша ==="
# Извлекаем чистый пароль, введенный пользователем
ADMIN_PASS=$(grep -E "^ADMIN_PASSWORD=" .env | cut -d'=' -f2-)

if [ -z "$ADMIN_PASS" ]; then
    echo "❌ Ошибка: Вы забыли добавить строчку ADMIN_PASSWORD=... в файл .env! Деплой остановлен."
    exit 1
fi

echo "⏳ Генерация криптографического хэша пароля..."
# Используем Docker Go, передавая переменные без участия sed
BCRYPT_HASH=$(docker run --rm -e PASS="$ADMIN_PASS" golang:1.25-alpine go run /dev/stdin 2>/dev/null << 'EOF'
package main
import (
    "fmt"
    "os"
    "golang.org/x/crypto/bcrypt"
)
func main() {
    p := os.Getenv("PASS")
    h, err := bcrypt.GenerateFromPassword([]byte(p), 10)
    if err != nil { os.Exit(1) }
    fmt.Print(string(h))
}
EOF
)

if [ -z "$BCRYPT_HASH" ]; then
    echo "❌ Ошибка: Хэш пароля пустой. Что-то пошло не так при генерации."
    exit 1
fi

# Полностью очищаем текстовый пароль из файла для безопасности
sudo sed -i '/^ADMIN_PASSWORD=/d' .env

# Безопасно ДОПИСЫВАЕМ хэш в конец файла обычным echo (ему не страшны спецсимволы и косые черты!)
echo "ADMIN_PASSWORD_HASH=$BCRYPT_HASH" >> .env

unset ADMIN_PASS
echo "✅ Текстовый пароль успешно удален и заменен на безопасный Bcrypt-хэш в конце .env."

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
