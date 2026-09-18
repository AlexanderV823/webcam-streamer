// Package stream управляет распределением кадров от видеоустройств к подключенным веб-слушателям.
package stream

import (
	"context"
	"log"
	"sync"
	"time"
	
	"webcam-streamer/internal/domain"
)

// Usecase инкапсулирует логику трансляции, смены источников и пула клиентов.
type Usecase struct {
	cam       domain.VideoCapture
	scanner   domain.CameraScanner
	currentID string
	listeners map[chan []byte]bool
	mu        sync.Mutex
}

// NewStreamUsecase теперь принимает интерфейсы бизнес-логики
func NewStreamUsecase(cam domain.VideoCapture, scanner domain.CameraScanner, defaultCamID string) *Usecase {
	return &Usecase{
		cam:       cam,
		scanner:   scanner,
		currentID: defaultCamID,
		listeners: make(map[chan []byte]bool),
	}
}

// StartBroadcast запускает бесконечный цикл захвата кадров
func (u *Usecase) StartBroadcast(ctx context.Context) {
	ticker := time.NewTicker(33 * time.Millisecond) // ~30 FPS
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			u.mu.Lock()
			frame, err := u.cam.ReadFrame()
			u.mu.Unlock()
			
			if err != nil || len(frame) == 0 {
				continue
			}

			u.mu.Lock()
			for ch := range u.listeners {
				select {
				case ch <- frame:
				default:
				}
			}
			u.mu.Unlock()
		}
	}
}

// SwitchCamera безопасно переключает источник видеопотока на лету (исправляет ошибку в handlers.go)
func (u *Usecase) SwitchCamera(newPath string) error {
	u.mu.Lock()
	defer u.mu.Unlock()

	if u.currentID == newPath {
		return nil
	}

	log.Printf("[STREAM] Переключение камеры с %s на %s", u.currentID, newPath)

	// Закрываем дескриптор старой камеры
	u.cam.Close()

	// Инициализируем новое устройство
	if err := u.cam.Init(newPath); err != nil {
		return err
	}

	u.currentID = newPath
	return nil
}

// ListAvailableCameras запрашивает список доступных в операционной системе камер через интерфейс сканера.
func (u *Usecase) ListAvailableCameras() ([]domain.DeviceInfo, error) {
	return u.scanner.Scan()
}

// AddListener создает и регистрирует новый канал для отправки кадров новому веб-клиенту.
func (u *Usecase) AddListener() chan []byte {
	u.mu.Lock()
	defer u.mu.Unlock()
	ch := make(chan []byte, 5)
	u.listeners[ch] = true
	return ch
}

// RemoveListener безопасно удаляет и закрывает канал клиента при его отключении.
func (u *Usecase) RemoveListener(ch chan []byte) {
	u.mu.Lock()
	defer u.mu.Unlock()
	if _, exists := u.listeners[ch]; exists {
		delete(u.listeners, ch)
		close(ch)
	}
}
