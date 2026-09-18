package middleware

import (
	"os"
	"testing"
)

func TestRotatingFileWriter_Workflow(t *testing.T) {
	tmpFile := "test_rotate_log.txt"
	defer os.Remove(tmpFile) // Гарантируем очистку после теста

	// Лимит 10 байт
	rotator := NewRotatingFileWriter(tmpFile, 10)

	// Тест 1: Обычная запись в пределах лимита
	msg1 := []byte("1234") // 4 байта
	n, err := rotator.Write(msg1)
	if err != nil || n != 4 {
		t.Fatalf("Ошибка первой записи: %v, записано %d байт", err, n)
	}

	// Тест 2: Запись, которая вызывает превышение лимита (4 + 8 = 12 > 10)
	msg2 := []byte("56789012") // 8 байт
	n, err = rotator.Write(msg2)
	if err != nil || n != 8 {
		t.Fatalf("Ошибка второй записи: %v, записано %d байт", err, n)
	}

	// Проверяем, что файл действительно очистился и теперь содержит ТОЛЬКО второе сообщение
	data, err := os.ReadFile(tmpFile)
	if err != nil {
		t.Fatalf("Не удалось прочитать файл лога: %v", err)
	}

	if string(data) != "56789012" {
		t.Errorf("Ожидалось, что файл будет перезаписан текстом '56789012', но получено '%s'", string(data))
	}
}
