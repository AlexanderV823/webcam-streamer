//go:build windows

package camera

import (
	"errors"

	"webcam-streamer/internal/domain"
)

// WindowsScanner реализует интерфейс domain.CameraScanner для операционной системы Windows.
type WindowsScanner struct{}

// WindowsCamera реализует интерфейс domain.VideoCapture для операционной системы Windows.
type WindowsCamera struct{}

// NewCamera инициализирует и возвращает Windows-реализацию интерфейса захвата видео.
func NewCamera() domain.VideoCapture {
	return &WindowsCamera{}
}

// NewScanner инициализирует и возвращает Windows-реализацию интерфейса сканирования устройств.
func NewScanner() domain.CameraScanner {
	return &WindowsScanner{}
}

// Scan возвращает список доступных видеокамер в Windows.
// На текущем этапе это эмуляционная заглушка. В будущем здесь будет вызов системных API
// Media Foundation или DirectShow через syscall.NewLazyDLL для загрузки mfplat.dll / ole32.dll.
func (s *WindowsScanner) Scan() ([]domain.DeviceInfo, error) {
	return []domain.DeviceInfo{
		{
			ID:   "DSHOW_VIRTUAL_CAM_0",
			Name: "Демо-камера Windows (Эмуляция DirectShow)",
		},
	}, nil
}

// Init открывает сессию захвата для выбранной камеры в Windows.
// На текущем этапе функция работает как мок-заглушка для успешной локальной компиляции на Windows-машине.
func (c *WindowsCamera) Init(path string) error {
	// Инициализация COM-компонентов и графа фильтров DirectShow будет добавлена здесь
	return nil
}

// ReadFrame считывает текущий кадр из видеобуфера операционной системы Windows.
// Пока реализация находится в процессе разработки, функция возвращает ошибку,
// предотвращая бесконечный пустой цикл вещания в UseCase.
func (c *WindowsCamera) ReadFrame() ([]byte, error) {
	return nil, errors.New("windows camera implementation (DirectShow/MediaFoundation) is not implemented yet")
}

// Close освобождает системные дескрипторы, COM-интерфейсы и закрывает сессию камеры в Windows.
func (c *WindowsCamera) Close() error {
	// Освобождение интерфейсов IMFMediaSource или IGraphBuilder будет добавлено здесь
	return nil
}
