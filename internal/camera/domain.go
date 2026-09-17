// Package camera содержит бизнес-сущности и контракты (интерфейсы) приложения,
// описывающие логику работы с веб-камерами без привязки к конкретным технологиям.
package camera

import "context"

// Camera описывает сущность видеокамеры в системе
type Camera struct {
	// ID содержит короткий идентификатор устройства (например, "video0").
	ID   string `json:"id"`
	// Path содержит полный системный путь к файлу устройства (например, "/dev/video0").
	Path string `json:"path"`
	// Name содержит читаемое имя камеры для отображения в интерфейсе.
	Name string `json:"name"`
}

// CameraUseCase задает контракт для слоя бизнес-логики (сценариев использования)
type CameraUseCase interface {
	StartStream(ctx context.Context, id string, width, height, fps int) (<-chan []byte, <-chan error, error)
}

// CameraRepository задает контракт для работы с хранилищем (базой данных или конфигом)
type CameraRepository interface {
	GetByID(ctx context.Context, id string) (*Camera, error)
}

// CameraStreamer задает контракт для низкоуровневого стримера (FFmpeg)
type CameraStreamer interface {
	Start(ctx context.Context, url string, width, height, fps int) (<-chan []byte, <-chan error, error)
}