//go:build windows

package camera

import "errors"

type WindowsCamera struct{}

func NewCamera() Device {
	return &WindowsCamera{}
}

// ScanDevices возвращает заглушку списка камер для Windows
func ScanDevices() ([]DeviceInfo, error) {
	// В будущем здесь будет вызов через DirectShow/Media Foundation COM API
	return []DeviceInfo{
		{ID: "COM1", Name: "Демо-камера Windows (Эмуляция)"},
	}, nil
}

func (c *WindowsCamera) Init(path string) error {
	return nil
}

func (c *WindowsCamera) ReadFrame() ([]byte, error) {
	return nil, errors.New("windows camera implementation (DirectShow) is not implemented yet")
}

func (c *WindowsCamera) Close() error {
	return nil
}
