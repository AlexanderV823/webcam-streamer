package embed

import (
	"testing"
)

// TestWebUI_Embedding проверяет, что HTML и CSS успешно вкомпилированы через go:embed.
func TestWebUI_Embedding(t *testing.T) {
	// Проверяем HTML
	htmlData, err := WebUI.ReadFile("index.html")
	if err != nil {
		t.Fatalf("Ошибка чтения встроенного файла index.html: %v", err)
	}
	if len(htmlData) == 0 {
		t.Error("Встроенный файл index.html пустой")
	}

	// Проверяем CSS
	cssData, err := WebUI.ReadFile("style.css")
	if err != nil {
		t.Fatalf("Ошибка чтения встроенного файла style.css: %v", err)
	}
	if len(cssData) == 0 {
		t.Error("Встроенный файл style.css пустой")
	}
}
