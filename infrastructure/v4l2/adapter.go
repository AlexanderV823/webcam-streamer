// Package v4l2 реализует интерфейсы слоя domain для операционных систем семейства Linux,
// взаимодействуя напрямую с драйверами Video for Linux 2.
package v4l2

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"webcam-streamer/domain"

	"github.com/vladimirvivien/go4vl/device"
	"github.com/vladimirvivien/go4vl/v4l2"
)

// V4L2Repository реализует поиск видеоустройств в файловой системе Linux.
type V4L2Repository struct{}

// NewV4L2Repository создает новый экземпляр V4L2Repository.
func NewV4L2Repository() *V4L2Repository {
	return &V4L2Repository{}
}

// List сканирует директорию /dev/ по шаблону video* и формирует список доменных структур камер.
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

// V4L2Streamer отвечает за открытие дескрипторов устройств и чтение видеобуфера ядра.
type V4L2Streamer struct{}

// NewV4L2Streamer создает новый экземпляр V4L2Streamer.
func NewV4L2Streamer() *V4L2Streamer {
	return &V4L2Streamer{}
}

// Start открывает камеру через вызовы V4L2 в формате MJPEG, настраивает геометрию кадра,
// FPS и запускает фоновую горутину для непрерывной прокачки кадров в канал передачи.
func (s *V4L2Streamer) Start(ctx context.Context, path string, width, height, fps int) (<-chan []byte, <-chan error, error) {
	// Инициализируем камеру, передавая все параметры (разрешение и FPS) через опции конструктора
	cam, err := device.Open(
		path,
		device.WithPixFormat(v4l2.PixFormat{
			PixelFormat: v4l2.PixelFmtMJPEG,
			Width:       uint32(width),
			Height:      uint32(height),
		}),
		device.WithFPS(uint32(fps)), // Нативная установка FPS через внутреннее API go4vl
	)
	if err != nil {
		return nil, nil, fmt.Errorf("v4l2 open failed: %w", err)
	}

	// Запускаем внутренний конвейер захвата видеопотока go4vl
	if err := cam.Start(ctx); err != nil {
		_ = cam.Close()
		return nil, nil, fmt.Errorf("v4l2 stream start failed: %w", err)
	}

	frameChan := make(chan []byte, 2)
	errChan := make(chan error, 1)

	// Получаем канал вывода кадров самой библиотеки
	go4vlOutput := cam.GetOutput()

	// Асинхронный конвейер перекачки кадров в транспортный слой
	go func() {
		defer func() {
			_ = cam.Close() // Закрытие устройства автоматически останавливает захват
			close(frameChan)
			close(errChan)
		}()

		for {
			select {
			case <-ctx.Done():
				return
			case frame, ok := <-go4vlOutput:
				if !ok {
					return
				}

				// Передаем кадр дальше, если HTTP-обработчик готов его принять
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
