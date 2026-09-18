package camera

// DeviceInfo хранит информацию о найденной камере
type DeviceInfo struct {
	ID   string `json:"id"`   // Для Linux: "/dev/video0", для Windows: Friendly Name / Symlink
	Name string `json:"name"` // Понятное имя устройства
}

// Device описывает абстрактное устройство захвата видеопотока
type Device interface {
	Init(path string) error
	ReadFrame() ([]byte, error)
	Close() error
}
