// Package domain содержит 핵심ные бизнес-модели, интерфейсы и общие сущности приложения.
package domain

// DeviceInfo описывает структуру данных камеры на уровне ядра
type DeviceInfo struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// CameraScanner определяет интерфейс для поиска камер в ОС
type CameraScanner interface {
	Scan() ([]DeviceInfo, error)
}

// VideoCapture определяет интерфейс для работы с конкретным устройством
type VideoCapture interface {
	Init(path string) error
	ReadFrame() ([]byte, error)
	Close() error
}
