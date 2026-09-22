package stream

import (
	"context"
	"errors"
	"testing"
	"time"

	"webcam-streamer/internal/domain"
)

// testCapture реализует domain.VideoCapture для использования в HTTP-тестах
type testCapture struct {
	returnErr      bool
	initErr        bool
	closeCalled    bool
	lastOpenedPath string
}

func (tc *testCapture) Init(path string) error {
	tc.lastOpenedPath = path
	if tc.initErr {
		return errors.New("init hardware fail")
	}
	return nil
}

func (tc *testCapture) ReadFrame() ([]byte, error) {
	if tc.returnErr {
		return nil, errors.New("hardware fail")
	}
	return []byte{0x01, 0x02}, nil
}

func (tc *testCapture) Close() error {
	tc.closeCalled = true
	return nil
}

type testScanner struct{}

func (ts *testScanner) Scan() ([]domain.DeviceInfo, error) {
	return []domain.DeviceInfo{{ID: "/dev/video0", Name: "Test Cam"}}, nil
}

// TestOriginalStreamWithInterfaces проверяет асинхронный стриминг и сканер устройств через интерфейсы
func TestOriginalStreamWithInterfaces(t *testing.T) {
	capture := &testCapture{}
	scanner := &testScanner{}

	streamUC := NewStreamUsecase(capture, scanner, "/dev/video0")

	// Проверяем работу сканера через интерфейс
	cams, err := streamUC.ListAvailableCameras()
	if err != nil || len(cams) != 1 {
		t.Error("Ошибка тестирования сканера через интерфейс")
	}

	// Проверяем асинхронный стриминг
	ch := streamUC.AddListener()
	ctx, cancel := context.WithCancel(context.Background())
	go streamUC.StartBroadcast(ctx)

	select {
	case frame := <-ch:
		if len(frame) == 0 {
			t.Error("Получен пустой кадр")
		}
	case <-time.After(150 * time.Millisecond): // Стабильный таймаут для Docker-контейнеров
		t.Error("Таймаут стрима")
	}
	cancel()
	streamUC.RemoveListener(ch)
}

// TestStartBroadcast_HardwareErrorHandling проверяет обработку аппаратных ошибок в цикле бродкаста
func TestStartBroadcast_HardwareErrorHandling(t *testing.T) {
	// Включаем возврат ошибки в mock-камере
	capture := &testCapture{returnErr: true}
	scanner := &testScanner{}
	streamUC := NewStreamUsecase(capture, scanner, "/dev/video0")

	ch := streamUC.AddListener()
	defer streamUC.RemoveListener(ch)

	ctx, cancel := context.WithCancel(context.Background())
	go streamUC.StartBroadcast(ctx)
	defer cancel()

	// Так как ReadFrame возвращает ошибку, кадры в канал идти не должны
	select {
	case <-ch:
		t.Error("Слушатель получил кадр, хотя камера вернула ошибку")
	case <-time.After(100 * time.Millisecond):
		// Успех: цикл корректно обработал ошибку через continue, не забив канал слушателя
	}
}

// TestSwitchCamera_Scenarios тестирует сценарии безопасной смены источников видео на лету
func TestSwitchCamera_Scenarios(t *testing.T) {
	capture := &testCapture{}
	scanner := &testScanner{}
	streamUC := NewStreamUsecase(capture, scanner, "/dev/video0")

	// Сценарий 1: Переключение на ту же самую камеру (должно выйти без переинициализации)
	err := streamUC.SwitchCamera("/dev/video0")
	if err != nil {
		t.Fatalf("Ошибка при переключении на текущую камеру: %v", err)
	}
	if capture.closeCalled {
		t.Error("Камера не должна была закрываться при переключении на саму себя")
	}

	// Сценарий 2: Успешная смена устройства
	err = streamUC.SwitchCamera("/dev/video1")
	if err != nil {
		t.Fatalf("Не удалось переключить камеру: %v", err)
	}
	if !capture.closeCalled {
		t.Error("Старая камера должна быть закрыта перед переключением")
	}
	if capture.lastOpenedPath != "/dev/video1" {
		t.Errorf("Ожидалось открытие '/dev/video1', открыто: %q", capture.lastOpenedPath)
	}

	// Сценарий 3: Обработка сбоя при инициализации новой камеры и принудительный откат на старую
	capture.initErr = true // Провоцируем ошибку Init при следующем переключении
	err = streamUC.SwitchCamera("/dev/video2")
	if err == nil {
		t.Error("Ожидалась ошибка инициализации оборудования, но метод вернул nil")
	}

	// Верифицируем, что UseCase успешно откатил и восстановил камеру на место после сбоя
	if streamUC.cam == nil {
		t.Error("После сбоя инициализации исходная камера должна была откатиться назад, но осталась nil")
	}
}

// TestRemoveListener_Safety тестирует удаление слушателей для исключения утечек памяти
func TestRemoveListener_Safety(t *testing.T) {
	capture := &testCapture{}
	scanner := &testScanner{}
	streamUC := NewStreamUsecase(capture, scanner, "/dev/video0")

	ch := streamUC.AddListener()

	// Проверяем, что в map добавился 1 элемент
	streamUC.mu.Lock()
	listenersCountBefore := len(streamUC.listeners)
	streamUC.mu.Unlock()
	if listenersCountBefore != 1 {
		t.Errorf("Ожидался 1 слушатель, найдено %d", listenersCountBefore)
	}

	streamUC.RemoveListener(ch)

	// Проверяем, что map пуста, а канал закрыт
	streamUC.mu.Lock()
	listenersCountAfter := len(streamUC.listeners)
	streamUC.mu.Unlock()
	if listenersCountAfter != 0 {
		t.Errorf("После удаления ожидалось 0 слушателей, найдено %d", listenersCountAfter)
	}

	// Повторное удаление несуществующего слушателя не должно вызывать паники
	streamUC.RemoveListener(ch)
}

// TestStartBroadcast_NilCameraSkip проверяет пропуск итерации бродкаста, если камера временно nil
func TestStartBroadcast_NilCameraSkip(t *testing.T) {
	scanner := &testScanner{}
	// Инициализируем UseCase БЕЗ камеры (nil)
	streamUC := NewStreamUsecase(nil, scanner, "/dev/video0")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Запуск не должен вызывать паники (nil pointer дедлок устранен)
	go streamUC.StartBroadcast(ctx)

	// Даем горутине прокрутиться несколько циклов мимо ветки activeCam == nil
	time.Sleep(40 * time.Millisecond)
}
