// Package config отвечает за чтение, парсинг и валидацию переменных окружения из .env файла.

package config

import (
	"errors"
	"os"
	"strings"
	"strconv"
)

// Config хранит конфигурацию приложения, необходимую для работы всех слоев.
type Config struct {
	Port         string
	Username     string
	PasswordHash string
	JWTSecret    string
	DefaultCam   string
	MaxLogSize   int64 // Размер в байтах
}

// Load считывает .env и валидирует данные, возвращая ошибку вместо жесткого падения процесса
func Load() (*Config, error) {
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

	port := getEnv("SERVER_PORT", "8080")
	username := os.Getenv("ADMIN_USERNAME")
	passwordHash := os.Getenv("ADMIN_PASSWORD_HASH")
	jwtSecret := os.Getenv("JWT_SECRET")
	defaultCam := getEnv("DEFAULT_CAMERA", "/dev/video0")
	
	// Читаем лимит логов из .env (в мегабайтах)
	maxLogSizeMBStr := getEnv("MAX_LOG_SIZE_MB", "5")
	maxLogSizeMB, err := strconv.ParseInt(maxLogSizeMBStr, 10, 64)
	if err != nil || maxLogSizeMB <= 0 {
		maxLogSizeMB = 5 // Дефолтное значение
	}

	if username == "" || passwordHash == "" || jwtSecret == "" {
		return nil, errors.New("критические переменные окружения отсутствуют")
	}

	// Проверяем длину JWT ключа для гарантированной стойкости HMAC-SHA256
	if len(jwtSecret) < 32 {
		return nil, errors.New("переменная JWT_SECRET слишком короткая (минимум 32 символа)")
	}

	return &Config{
		Port:         port,
		Username:     username,
		PasswordHash: passwordHash,
		JWTSecret:    jwtSecret,
		DefaultCam:   defaultCam,
		MaxLogSize:   maxLogSizeMB * 1024 * 1024, // Конвертируем в байты
	}, nil
}

func getEnv(key, fallback string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return fallback
}
