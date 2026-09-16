// Package domain содержит бизнес-сущности и контракты (интерфейсы) приложения,
// описывающие логику работы с веб-камерами без привязки к конкретным технологиям.
package domain

import "context"

// Camera описывает доменную сущность физического или виртуального USB-устройства.
type Camera struct {
	// ID содержит короткий идентификатор устройства (например, "video0").
	ID   string `json:"id"`
	// Path содержит полный системный путь к файлу устройства (например, "/dev/video0").
	Path string `json:"path"`
	// Name содержит читаемое имя камеры для отображения в интерфейсе.
	Name string `json:"name"`
}

// CameraRepository определяет контракт для сканирования и получения списка камер в системе.
type CameraRepository interface {
	// List возвращает срез всех доступных видеоустройств, обнаруженных в операционной системе.
	List(ctx context.Context) ([]Camera, error)
}

// CameraStreamer определяет контракт для низкоуровневого захвата видеопотока.
type CameraStreamer interface {
	// Start инициализирует захват кадров с устройства по указанному пути с заданным качеством
	// и возвращает каналы для чтения бинарных кадров и отслеживания ошибок.
	Start(ctx context.Context, path string, width, height, fps int) (<-chan []byte, <-chan error, error)
}
