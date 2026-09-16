package v4l2

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"webcam-streamer/domain"

	"github.com/vladimirvivien/go4vl"
)

type V4L2Repository struct{}

func NewV4L2Repository() *V4L2Repository { return &V4L2Repository{} }

func (r *V4L2Repository) List(ctx context.Context) ([]domain.Camera, error) {
	matches, err := filepath.Glob("/dev/video*")
	if err != nil {
		return nil, fmt.Errorf("failed to scan v4l2 devices: %w", err)
	}

	var cameras []domain.Camera
	for _, match := range matches {
		id := strings.TrimPrefix(match, "/dev/")
		cameras = append(cameras, domain.Camera{
			ID:   id,
			Path: match,
			Name: "USB Camera " + id,
		})
	}
	return cameras, nil
}

type V4L2Streamer struct{}

func NewV4L2Streamer() *V4L2Streamer { return &V4L2Streamer{} }

func (s *V4L2Streamer) Start(ctx context.Context, path string, width, height, fps int) (<-chan []byte, <-chan error, error) {
	// Инициализируем камеру с динамическим разрешением
	cam, err := v4l2.Init(path, uint32(width), uint32(height), v4l2.MJPEG)
	if err != nil {
		return nil, nil, fmt.Errorf("v4l2 init failed: %w", err)
	}

	// Попытка принудительно выставить FPS
	if err := cam.SetFps(uint32(fps)); err != nil {
		// Некоторые дешевые камеры выбрасывают ошибку, если не поддерживают смену FPS.
		// Логируем, но не прерываем работу.
		fmt.Printf("⚠️ Предупреждение: Камера не поддерживает установку FPS %d: %v\n", fps, err)
	}

	if err := cam.Start(); err != nil {
		cam.Close()
		return nil, nil, fmt.Errorf("v4l2 start failed: %w", err)
	}

	frameChan := make(chan []byte)
	errChan := make(chan error, 1)

	// Асинхронный конвейер чтения кадров
	go func() {
		defer func() {
			cam.Stop()
			cam.Close()
			close(frameChan)
			close(errChan)
		}()

		for {
			select {
			case <-ctx.Done():
				return
			default:
				frame, err := cam.Read()
				if err != nil {
					errChan <- err
					return
				}

				// Передаем кадр в канал, если есть читатель
				select {
				case frameChan <- frame:
				case <-ctx.Done():
					return
				}
			}
		}
	}()

	return frameChan, errChan, nil
}
