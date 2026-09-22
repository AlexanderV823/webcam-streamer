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
			// 1. Безопасно извлекаем указатель на текущую активную камеру
			u.mu.Lock()
			activeCam := u.cam
			u.mu.Unlock()

			// Если камера в данный момент закрыта или переключается — пропускаем итерацию
			if activeCam == nil {
				continue
			}

			// 2. ВАЖНО: Читаем кадр БЕЗ блокировки u.mu.Lock().
			// Теперь медленные системные вызовы WaitForFrame не тормозят UseCase!
			frame, err := activeCam.ReadFrame()
			if err != nil || len(frame) == 0 {
				continue
			}

			// 3. Быстро блокируем мьютекс только для рассылки по карте каналов
			u.mu.Lock()
			for ch := range u.listeners {
				select {
				case ch <- frame:
				default:
					// Пропускаем медленных клиентов (drop frame)
				}
			}
			u.mu.Unlock()
		}
	}
}

// SwitchCamera безопасно переключает источник видеопотока на лету
func (u *Usecase) SwitchCamera(newPath string) error {
	u.mu.Lock()
	if u.currentID == newPath {
		u.mu.Unlock()
		return nil
	}

	log.Printf("[STREAM] Переключение камеры с %s на %s", u.currentID, newPath)

	// Запоминаем ссылку на старое устройство и временно зануляем u.cam,
	// чтобы горутина StartBroadcast временно пропускала итерации захвата
	oldCam := u.cam
	u.cam = nil
	u.mu.Unlock()

	// Закрываем дескриптор старой камеры вне мьютекса
	if oldCam != nil {
		oldCam.Close()
	}

	// Инициализируем новое устройство
	if err := oldCam.Init(newPath); err != nil {
		// В случае ошибки возвращаем старую камеру на место
		u.mu.Lock()
		u.cam = oldCam
		u.mu.Unlock()
		return err
	}

	u.mu.Lock()
	u.cam = oldCam
	u.currentID = newPath
	u.mu.Unlock()
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
