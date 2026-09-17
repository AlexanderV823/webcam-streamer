// Package usecase реализует сценарии использования приложения (бизнес-логику),
// оркестрируя выполнение операций между транспортным и инфраструктурным слоями.
package usecase

import (
	"context"
	"fmt"
	"webcam-streamer/internal/camera"
)

// CameraUseCase реализует интерфейс camera.CameraUseCase
type CameraUseCase struct {
	repo     camera.CameraRepository
	streamer camera.CameraStreamer
}

// NewCameraUseCase создает новый экземпляр бизнес-логики
func NewCameraUseCase(repo camera.CameraRepository, streamer camera.CameraStreamer) *CameraUseCase {
	return &CameraUseCase{
		repo:     repo,
		streamer: streamer,
	}
}

// StartStream реализует бизнес-логику запуска трансляции
func (uc *CameraUseCase) StartStream(ctx context.Context, id string, width, height, fps int) (<-chan []byte, <-chan error, error) {
	var streamURL string
	if uc.repo != nil {
		cam, err := uc.repo.GetByID(ctx, id)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to get camera: %w", err)
		}
		streamURL = cam.Path // Используем только существующее поле Path
	} else {
		streamURL = id 
	}

	return uc.streamer.Start(ctx, streamURL, width, height, fps)
}
