// usecase/camera_uc_test.go
package usecase

import (
	"context"
	"testing"
	"webcam-streamer/domain"
)

// mockRepo симулирует поведение репозитория камер без сканирования /dev/
type mockRepo struct {
	cameras []domain.Camera
	err     error
}

func (m *mockRepo) List(ctx context.Context) ([]domain.Camera, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.cameras, nil
}

// mockStreamer симулирует захват видеопотока без системных вызовов V4L2
type mockStreamer struct {
	frameChan chan []byte
	errChan   chan error
	err       error
}

func (m *mockStreamer) Start(ctx context.Context, path string, width, height, fps int) (<-chan []byte, <-chan error, error) {
	if m.err != nil {
		return nil, nil, m.err
	}
	return m.frameChan, m.errChan, nil
}

// TestGetAvailableCameras проверяет успешный сценарий получения списка камер
func TestGetAvailableCameras_Success(t *testing.T) {
	// Подготовка тестовых данных
	expectedCameras := []domain.Camera{
		{ID: "video0", Path: "/dev/video0", Name: "USB Camera video0"},
	}
	repo := &mockRepo{cameras: expectedCameras}
	streamer := &mockStreamer{}

	// Инициализация тестируемого UseCase
	uc := NewCameraUseCase(repo, streamer)

	// Выполнение сценария
	res, err := uc.GetAvailableCameras(context.Background())

	// Проверка результатов
	if err != nil {
		t.Fatalf("Ожидался успешный результат, получен код ошибки: %v", err)
	}
	if len(res) != 1 || res[0].ID != "video0" {
		t.Errorf("Получены некорректные данные камер: %v", res)
	}
}

// TestGetStream_EmptyPath проверяет валидацию пустой строки пути к камере
func TestGetStream_EmptyPath(t *testing.T) {
	repo := &mockRepo{}
	streamer := &mockStreamer{}
	uc := NewCameraUseCase(repo, streamer)

	// Передаем пустую строку пути
	_, _, err := uc.GetStream(context.Background(), "", 640, 480, 30)

	// Логика должна упасть на валидации до обращения к драйверам
	if err == nil {
		t.Error("Ожидалась ошибка валидации пустого пути, но метод выполнился успешно")
	}
	if err.Error() != "camera path cannot be empty" {
		t.Errorf("Ожидался текст ошибки 'camera path cannot be empty', получено: %q", err.Error())
	}
}

// TestGetStream_Success проверяет успешную инициализацию каналов потока
func TestGetStream_Success(t *testing.T) {
	repo := &mockRepo{}
	// Инициализируем фейковые каналы данных
	fChan := make(chan []byte)
	eChan := make(chan error, 1)
	streamer := &mockStreamer{frameChan: fChan, errChan: eChan}
	
	uc := NewCameraUseCase(repo, streamer)

	// Запрашиваем стрим
	frameOut, errOut, err := uc.GetStream(context.Background(), "/dev/video0", 640, 480, 30)

	if err != nil {
		t.Fatalf("Ожидался успешный запуск потока, получена ошибка: %v", err)
	}
	if frameOut == nil || errOut == nil {
		t.Error("Каналы вывода потока не должны быть nil")
	}
}
