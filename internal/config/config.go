package config

import (
	"log"
	"os"
	"strings"
)

type Config struct {
	Port         string
	Username     string
	PasswordHash string
	JWTSecret    string
	DefaultCam   string
}

func Load() *Config {
	// Примитивный парсинг .env, если он существует
	if data, err := os.ReadFile(".env"); err == nil {
		lines := strings.Split(string(data), "\n")
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if line == "" || strings.HasPrefix(line, "#") {
				continue
			}
			parts := strings.SplitN(line, "=", 2)
			if len(parts) == 2 {
				os.Setenv(strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1]))
			}
		}
	}

	// Считываем переменные окружения
	port := getEnv("SERVER_PORT", "8080") // Для порта дефолтное значение оставить безопасно
	username := os.Getenv("ADMIN_USERNAME")
	passwordHash := os.Getenv("ADMIN_PASSWORD_HASH")
	jwtSecret := os.Getenv("JWT_SECRET")
	defaultCam := getEnv("DEFAULT_CAMERA", "/dev/video0") // Для камеры тоже допустимо

	// ЖЕСТКАЯ ПРОВЕРКА БЕЗОПАСНОСТИ: Если секреты не заданы — останавливаем приложение
	if username == "" || passwordHash == "" || jwtSecret == "" {
		log.Fatal("[CRITICAL SECURITY ERROR] Критические переменные окружения (ADMIN_USERNAME, ADMIN_PASSWORD_HASH, JWT_SECRET) должны быть обязательно настроены в .env файле перед деплоем!")
	}

	// Проверяем длину JWT ключа для гарантированной стойкости HMAC-SHA256
	if len(jwtSecret) < 32 {
		log.Fatal("[CRITICAL SECURITY ERROR] Переменная JWT_SECRET слишком короткая! Длина должна быть не менее 32 символов для защиты от перебора подписей.")
	}

	return &Config{
		Port:         port,
		Username:     username,
		PasswordHash: passwordHash,
		JWTSecret:    jwtSecret,
		DefaultCam:   defaultCam,
	}
}

func getEnv(key, fallback string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return fallback
}
