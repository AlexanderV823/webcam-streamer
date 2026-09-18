package stream

import (
	"context"
	"log"
	"sync"
	"time"

	"webcam-streamer/pkg/camera"
)

type Usecase struct {
	cam       camera.Device
	currentID string
	listeners map[chan []byte]bool
	mu        sync.Mutex
}

// NewStreamUsecase теперь принимает два аргумента: устройство и ID дефолтной камеры
func NewStreamUsecase(cam camera.Device, defaultCamID string) *Usecase {
	return &Usecase{
		cam:       cam,
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
		return nil // Эта камера уже активна
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

func (u *Usecase) AddListener() chan []byte {
	u.mu.Lock()
	defer u.mu.Unlock()
	ch := make(chan []byte, 5)
	u.listeners[ch] = true
	return ch
}

func (u *Usecase) RemoveListener(ch chan []byte) {
	u.mu.Lock()
	defer u.mu.Unlock()
	if _, exists := u.listeners[ch]; exists {
		delete(u.listeners, ch)
		close(ch)
	}
}
