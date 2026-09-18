package stream

import (
	"testing"
)

// Описываем mock-камеру для изоляции тестов от ОС
type mockCamera struct{}

func (m *mockCamera) Init(path string) error       { return nil }
func (m *mockCamera) ReadFrame() ([]byte, error) { return []byte("fake-jpeg"), nil }
func (m *mockCamera) Close() error              { return nil }

func TestStreamUsecase_Listeners(t *testing.T) {
	cam := &mockCamera{}
	uc := NewStreamUsecase(cam, "/dev/video0")

	// 1. Проверяем регистрацию слушателя
	ch := uc.AddListener()
	if ch == nil {
		t.Fatalf("Канал слушателя не инициализирован")
	}

	uc.mu.Lock()
	listenersCount := len(uc.listeners)
	uc.mu.Unlock()

	if listenersCount != 1 {
		t.Errorf("Ожидался 1 активный слушатель, найдено: %d", listenersCount)
	}

	// 2. Проверяем корректное удаление слушателя
	uc.RemoveListener(ch)

	uc.mu.Lock()
	listenersCount = len(uc.listeners)
	uc.mu.Unlock()

	if listenersCount != 0 {
		t.Errorf("Пул слушателей должен быть пуст после удаления, найдено: %d", listenersCount)
	}
}
