package camera

import (
	"testing"
)

// TestCameraInfrastructure проверяет работоспособность фабричных методов и сканера ОС.
func TestCameraInfrastructure(t *testing.T) {
	// Проверяем инициализацию сканера
	scanner := NewScanner()
	if scanner == nil {
		t.Fatal("Сканер камер не должен быть nil")
	}

	// Запускаем сканирование (в Windows вернет мок, в Linux отсканирует /dev)
	devices, err := scanner.Scan()
	if err != nil {
		t.Fatalf("Ошибка при сканировании устройств: %v", err)
	}

	// Проверяем, что возвращается срез данных (даже если он пустой в Linux без камер)
	if devices == nil {
		t.Error("Срез устройств не должен быть nil")
	}

	// Проверяем инициализацию драйвера камеры
	cam := NewCamera()
	if cam == nil {
		t.Error("Драйвер камеры не должен быть nil")
	}
}
