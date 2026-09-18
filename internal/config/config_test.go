package config

import (
	"os"
	"testing"
)

func TestGetEnv(t *testing.T) {
	// Устанавливаем тестовую переменную окружения
	os.Setenv("TEST_WEBCAM_VAR", "active")
	defer os.Unsetenv("TEST_WEBCAM_VAR")

	// Тест 1: Переменная существует
	if val := getEnv("TEST_WEBCAM_VAR", "default"); val != "active" {
		t.Errorf("Ожидалось значение 'active', но получено '%s'", val)
	}

	// Тест 2: Переменная отсутствует, должен вернуться fallback
	if val := getEnv("NON_EXISTENT_VAR", "fallback_val"); val != "fallback_val" {
		t.Errorf("Ожидался fallback 'fallback_val', но получено '%s'", val)
	}
}
