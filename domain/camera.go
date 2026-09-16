package domain

import "context"

type Camera struct {
	ID   string `json:"id"`
	Path string `json:"path"`
	Name string `json:"name"`
}

// CameraRepository описывает, как мы ищем камеры в системе
type CameraRepository interface {
	List(ctx context.Context) ([]Camera, error)
}

// CameraStreamer описывает интерфейс захвата потока
type CameraStreamer interface {
	Start(ctx context.Context, path string, width, height, fps int) (<-chan []byte, <-chan error, error)
}
