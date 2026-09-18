package config

import (
	"os"
	"testing"
)

func TestLoad_SuccessAndFailure(t *testing.T) {
	// Сценарий 1: Ошибка валидации секретов
	os.Clearenv()
	_, err := Load()
	if err == nil {
		t.Error("Ожидалась ошибка при отсутствии переменных окружения")
	}

	// Сценарий 2: Ошибка короткого JWT
	os.Setenv("ADMIN_USERNAME", "admin")
	os.Setenv("ADMIN_PASSWORD_HASH", "hash")
	os.Setenv("JWT_SECRET", "short")
	_, err = Load()
	if err == nil {
		t.Error("Ожидалась ошибка стойкости для короткого JWT_SECRET")
	}

	// Сценарий 3: Успешная загрузка конфигурации
	os.Setenv("JWT_SECRET", "super-secure-secret-key-32-bytes-long!!!")
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Не удалось загрузить валидный конфиг: %v", err)
	}
	if cfg.Port != "8080" {
		t.Errorf("Ожидался порт 8080, получен %s", cfg.Port)
	}
}

func TestGetEnv(t *testing.T) {
	// Устанавливаем тестовую переменную окружения
	os.Setenv("TEST_WEBCAM_VAR", "active")
	defer os.Unsetenv("TEST_WEBCAM_VAR")

	// Тест 1: Переменная существует
	if val := getEnv("TEST_WEBCAM_VAR", "default"); val != "active" {
		t.Errorf("Ожидалось 'active', получено '%s'", val)
	}
}
