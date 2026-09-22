package middleware

import (
	"os"
	"path/filepath"
	"strings"
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

// TestRotatingFileWriter_Write_Normal проверяет обычную запись в режиме добавления (Append)
func TestRotatingFileWriter_Write_Normal(t *testing.T) {
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "test_normal.log")

	// Инициализируем ваш честный RotatingFileWriter с запасом по размеру
	var maxSize int64 = 1000
	writer := NewRotatingFileWriter(logPath, maxSize)

	// Пишем первую строку
	line1 := []byte("First line\n")
	n, err := writer.Write(line1)
	if err != nil {
		t.Fatalf("Ошибка при первой записи: %v", err)
	}
	if n != len(line1) {
		t.Errorf("Ожидалось записать %d байт, записано %d", len(line1), n)
	}

	// Пишем вторую строку (должна добавиться в конец через O_APPEND)
	line2 := []byte("Second line\n")
	_, err = writer.Write(line2)
	if err != nil {
		t.Fatalf("Ошибка при второй записи: %v", err)
	}

	// Считываем результат и проверяем, что обе строки на месте
	content, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("Не удалось прочитать файл лога: %v", err)
	}

	expected := string(line1) + string(line2)
	if string(content) != expected {
		t.Errorf("Содержимое не совпадает в режиме Append.\nОжидалось:\n%s\nПолучено:\n%s", expected, string(content))
	}
}

// TestRotatingFileWriter_Write_TriggerRotation проверяет очистку файла (O_TRUNC)
// при превышении лимита maxSize
func TestRotatingFileWriter_Write_TriggerRotation(t *testing.T) {
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "test_rotation.log")

	// Задаем лимит ровно в 30 байт
	var maxSize int64 = 30
	writer := NewRotatingFileWriter(logPath, maxSize)

	// 1. Первая запись занимает 20 байт (остается 10 байт лимита)
	firstPart := []byte("Log message 20 bytes")
	_, err := writer.Write(firstPart)
	if err != nil {
		t.Fatalf("Ошибка первой записи: %v", err)
	}

	// 2. Вторая запись весит 15 байт. 20 + 15 = 35 байт (это больше лимита в 30)
	// Должен сработать ваш триггер перезаписи с флагом O_TRUNC
	secondPart := []byte("Next 15 bytes!!")
	_, err = writer.Write(secondPart)
	if err != nil {
		t.Fatalf("Ошибка второй записи (триггер ротации): %v", err)
	}

	// 3. Проверяем содержимое файла на диске
	content, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("Не удалось прочитать лог после ротации: %v", err)
	}

	// Файл должен содержать ТОЛЬКО вторую строку, старая должна полностью стереться
	if string(content) != string(secondPart) {
		t.Errorf("Атомарная ротация не сработала. Ожидалось: %q, получено: %q", string(secondPart), string(content))
	}

	if strings.Contains(string(content), string(firstPart)) {
		t.Errorf("Критическая ошибка: старые логи остались внутри файла после пробития лимита!")
	}

	// Проверяем физический размер файла на диске
	fi, err := os.Stat(logPath)
	if err != nil {
		t.Fatalf("Не удалось получить свойства файла: %v", err)
	}

	if fi.Size() > maxSize {
		t.Errorf("Размер файла (%d байт) превышает лимит (%d байт)", fi.Size(), maxSize)
	}
}
