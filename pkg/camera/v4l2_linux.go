//go:build linux

package camera

import (
	"fmt"
	"os"
	"strings"
	"syscall"
	"unsafe"
)

const (
	v4l2BufferTypeVideoCapture = 1
	vidiocStreamOn             = 0x4004564a
	vidiocStreamOff            = 0x4004564b
)

type LinuxCamera struct {
	file *os.File
}

func NewCamera() Device {
	return &LinuxCamera{}
}

// ScanDevices сканирует систему на наличие подключенных USB-камер в Linux
func ScanDevices() ([]DeviceInfo, error) {
	var devices []DeviceInfo

	files, err := os.ReadDir("/dev")
	if err != nil {
		return nil, fmt.Errorf("не удалось прочитать директорию /dev: %w", err)
	}

	for _, file := range files {
		name := file.Name()
		// Нас интересуют только устройства захвата videoX (пропускаем video-subdev или другие)
		if strings.HasPrefix(name, "video") && !strings.Contains(name, "-") {
			devPath := "/dev/" + name

			// Пытаемся прочитать красивое имя камеры из sysfs Linux
			friendlyName := "Универсальная USB-камера (" + name + ")"
			sysNamePath := fmt.Sprintf("/sys/class/video4linux/%s/name", name)
			if nameBytes, err := os.ReadFile(sysNamePath); err == nil {
				friendlyName = strings.TrimSpace(string(nameBytes))
			}

			devices = append(devices, DeviceInfo{
				ID:   devPath,
				Name: friendlyName,
			})
		}
	}

	return devices, nil
}

func (c *LinuxCamera) Init(path string) error {
	f, err := os.OpenFile(path, os.O_RDWR|syscall.S_NONBLOCK, 0)
	if err != nil {
		return fmt.Errorf("не удалось открыть устройство камеры %s: %w", path, err)
	}
	c.file = f

	var bufType uint32 = v4l2BufferTypeVideoCapture
	_, _, errno := syscall.Syscall(syscall.SYS_IOCTL, c.file.Fd(), vidiocStreamOn, uintptr(unsafe.Pointer(&bufType)))
	if errno != 0 && errno != syscall.EBUSY {
		c.file.Close()
		return fmt.Errorf("ошибка ioctl VIDIOC_STREAMON: %w", errno)
	}
	return nil
}

func (c *LinuxCamera) ReadFrame() ([]byte, error) {
	if c.file == nil {
		return nil, fmt.Errorf("камера не инициализирована")
	}

	buf := make([]byte, 1024*500)
	n, err := c.file.Read(buf)
	if err != nil {
		if perr, ok := err.(*os.PathError); ok && perr.Err == syscall.EAGAIN {
			return nil, nil
		}
		return nil, err
	}
	return buf[:n], nil
}

func (c *LinuxCamera) Close() error {
	if c.file != nil {
		var bufType uint32 = v4l2BufferTypeVideoCapture
		syscall.Syscall(syscall.SYS_IOCTL, c.file.Fd(), vidiocStreamOff, uintptr(unsafe.Pointer(&bufType)))
		return c.file.Close()
	}
	return nil
}
