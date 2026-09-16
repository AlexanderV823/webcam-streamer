// Package usecase реализует сценарии использования приложения (бизнес-логику),
// оркестрируя выполнение операций между транспортным и инфраструктурным слоями.
package usecase

import (
	"context"
	"errors"
	"webcam-streamer/domain"
)

// CameraUseCase инкапсулирует логику управления камерами и потоками.
type CameraUseCase struct {
	repo     domain.CameraRepository
	streamer domain.CameraStreamer
}

// NewCameraUseCase выступает в роли конструктора для создания CameraUseCase
// с внедрением необходимых зависимостей (репозитория и стримера).
func NewCameraUseCase(r domain.CameraRepository, s domain.CameraStreamer) *CameraUseCase {
	return &CameraUseCase{repo: r, streamer: s}
}

// GetAvailableCameras возвращает список всех подключенных к серверу камер,
// запрашивая данные у нижележащего репозитория.
func (uc *CameraUseCase) GetAvailableCameras(ctx context.Context) ([]domain.Camera, error) {
	return uc.repo.List(ctx)
}

// GetStream проверяет корректность входных параметров и запрашивает асинхронный запуск
// видеотрансляции с указанными настройками разрешения и частоты кадров.
func (uc *CameraUseCase) GetStream(ctx context.Context, path string, width, height, fps int) (<-chan []byte, <-chan error, error) {
	if path == "" {
		return nil, nil, errors.New("camera path cannot be empty")
	}
	return uc.streamer.Start(ctx, path, width, height, fps)
}
