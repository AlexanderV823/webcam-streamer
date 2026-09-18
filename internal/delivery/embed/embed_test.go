package embed

import (
	"testing"
)

// TestWebUI_Embedding проверяет, что HTML-фронтенд успешно вкомпилирован через go:embed.
func TestWebUI_Embedding(t *testing.T) {
	// Пытаемся прочитать файл из встроенной файловой системы
	data, err := WebUI.ReadFile("index.html")
	if err != nil {
		t.Fatalf("Ошибка чтения встроенного файла index.html: %v", err)
	}

	if len(data) == 0 {
		t.Error("Встроенный файл index.html пустой")
	}
}
