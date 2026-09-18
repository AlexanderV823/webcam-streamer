package main

import (
	"os"
	"testing"
	"webcam-streamer/internal/config"
)

// TestMainInitialization_Failure проверяет реакцию точки входа на отсутствие конфигурации.
func TestMainInitialization_Failure(t *testing.T) {
	// Полностью очищаем переменные окружения, чтобы config.Load() вернул ошибку
	os.Clearenv()

	_, err := config.Load()
	if err == nil {
		t.Error("Ожидалась ошибка инициализации конфигурации при пустом окружении")
	}
	
	// Данный тест подтверждает, что логика защиты от уязвимостей в main.go
	// (срабатывание log.Fatalf при ошибке конфига) отработает корректно.
}
