package stream

import (
	"context"
	"errors"
	"testing"
	"time"

	"webcam-streamer/internal/domain"
)

// Тестовые структуры, реализующие интерфейсы ядра
type testCapture struct{ returnErr bool }

func (tc *testCapture) Init(_ string) error { return nil }
func (tc *testCapture) ReadFrame() ([]byte, error) {
	if tc.returnErr {
		return nil, errors.New("hardware fail")
	}
	return []byte{0x01, 0x02}, nil
}
func (tc *testCapture) Close() error { return nil }

type testScanner struct{}

func (ts *testScanner) Scan() ([]domain.DeviceInfo, error) {
	return []domain.DeviceInfo{{ID: "/dev/video0", Name: "Test Cam"}}, nil
}

func TestStreamWithInterfaces(t *testing.T) {
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
	case <-time.After(100 * time.Millisecond):
		t.Error("Таймаут стрима")
	}
	cancel()
	streamUC.RemoveListener(ch)
}
