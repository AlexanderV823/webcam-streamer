package usecase

import (
	"context"
	"errors"
	"webcam-streamer/domain"
)

type CameraUseCase struct {
	repo     domain.CameraRepository
	streamer domain.CameraStreamer
}

func NewCameraUseCase(r domain.CameraRepository, s domain.CameraStreamer) *CameraUseCase {
	return &CameraUseCase{repo: r, streamer: s}
}

func (uc *CameraUseCase) GetAvailableCameras(ctx context.Context) ([]domain.Camera, error) {
	return uc.repo.List(ctx)
}

func (uc *CameraUseCase) GetStream(ctx context.Context, path string) (<-chan []byte, <-chan error, error) {
	if path == "" {
		return nil, nil, errors.New("camera path cannot be empty")
	}
	return uc.streamer.Start(ctx, path)
}
