package domain

import "context"

type Camera struct {
	ID   string `json:"id"`   // Например: "video0"
	Path string `json:"path"` // Например: "/dev/video0"
	Name string `json:"name"` // Читаемое имя
}

// CameraRepository описывает, как мы ищем камеры в системе
type CameraRepository interface {
	List(ctx context.Context) ([]Camera, error)
}

// CameraStreamer описывает интерфейс захвата потока
type CameraStreamer interface {
	Start(ctx context.Context, path string) (<-chan []byte, <-chan error, error)
}
