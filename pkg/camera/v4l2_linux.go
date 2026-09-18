//go:build linux

package camera

import (
	"fmt"
	"os"
	"strings"
	"syscall"
	"unsafe"

	"webcam-streamer/internal/domain"
)

const (
	// v4l2BufferTypeVideoCapture указывает ядру Linux, что буфер используется для захвата видео
	v4l2BufferTypeVideoCapture = 1
	// vidiocStreamOn — код системного вызова ioctl для запуска трансляции с камеры
	vidiocStreamOn             = 0x4004564a
	// vidiocStreamOff — код системного вызова ioctl для остановки трансляции с камеры
	vidiocStreamOff            = 0x4004564b
)

// LinuxScanner реализует интерфейс domain.CameraScanner для операционной системы Linux.
type LinuxScanner struct{}

// LinuxCamera реализует интерфейс domain.VideoCapture для прямого взаимодействия с V4L2 без CGO.
type LinuxCamera struct {
	file *os.File
}

// NewCamera инициализирует и возвращает Linux-реализацию интерфейса захвата видео.
func NewCamera() domain.VideoCapture {
	return &LinuxCamera{}
}

// NewScanner инициализирует и возвращает Linux-реализацию интерфейса сканирования устройств.
func NewScanner() domain.CameraScanner {
	return &LinuxScanner{}
}

// Scan считывает системную директорию /dev и sysfs в Linux для поиска всех подключенных USB-камер.
// Она сопоставляет технические пути (например, /dev/video0) с их человекочитаемыми именами из ядра.
func (s *LinuxScanner) Scan() ([]domain.DeviceInfo, error) {
	var devices []domain.DeviceInfo

	// Читаем список файлов устройств в системе
	files, err := os.ReadDir("/dev")
	if err != nil {
		return nil, fmt.Errorf("не удалось прочитать директорию /dev: %w", err)
	}

	for _, file := range files {
		name := file.Name()
		// Отфильтровываем только основные файлы видеоустройств videoX, игнорируя subdev и метаданные
		if strings.HasPrefix(name, "video") && !strings.Contains(name, "-") {
			devPath := "/dev/" + name

			// Задаем базовое имя на случай, если sysfs не вернет красивое название
			friendlyName := "Универсальная USB-камера (" + name + ")"
			sysNamePath := fmt.Sprintf("/sys/class/video4linux/%s/name", name)
			
			// Пытаемся прочитать реальное коммерческое название камеры (например, Logitech) из метаданных ядра
			if nameBytes, err := os.ReadFile(sysNamePath); err == nil {
				friendlyName = strings.TrimSpace(string(nameBytes))
			}

			devices = append(devices, domain.DeviceInfo{
				ID:   devPath,
				Name: friendlyName,
			})
		}
	}

	return devices, nil
}

// Init открывает дескриптор файла USB-устройства в неблокирующем режиме (NONBLOCK)
// и через ioctl передает драйверу ядра Linux (V4L2) сигнал запустить видеопоток.
func (c *LinuxCamera) Init(path string) error {
	// Открываем файл устройства. NONBLOCK нужен, чтобы Read не зависал, если кадр задерживается
	f, err := os.OpenFile(path, os.O_RDWR|syscall.S_NONBLOCK, 0)
	if err != nil {
		return fmt.Errorf("не удалось открыть устройство камеры %s: %w", path, err)
	}
	c.file = f

	// Отправляем команду VIDIOC_STREAMON в ядро Linux
	var bufType uint32 = v4l2BufferTypeVideoCapture
	_, _, errno := syscall.Syscall(syscall.SYS_IOCTL, c.file.Fd(), vidiocStreamOn, uintptr(unsafe.Pointer(&bufType)))
	
	// EBUSY означает, что камера уже стримит (например, запущена другой программой) — в данном контексте это допустимо
	if errno != 0 && errno != syscall.EBUSY {
		c.file.Close()
		return fmt.Errorf("ошибка ioctl VIDIOC_STREAMON: %w", errno)
	}
	return nil
}

// ReadFrame считывает сырые байты текущего кадра из открытого файла устройства.
// Если камера переведена ядром в формат MJPEG, функция возвращает валидный сжатый JPEG-кадр.
func (c *LinuxCamera) ReadFrame() ([]byte, error) {
	if c.file == nil {
		return nil, fmt.Errorf("камера не инициализирована")
	}

	// Выделяем 500 КБ буфер, чего с запасом хватает для сжатого JPEG кадра в Full HD
	buf := make([]byte, 1024*500)
	n, err := c.file.Read(buf)
	if err != nil {
		// EAGAIN сигнализирует, что новый кадр на USB-шине еще просто не успел сформироваться.
		// Это штатное поведение для неблокирующего IO, ошибкой не является.
		if perr, ok := err.(*os.PathError); ok && perr.Err == syscall.EAGAIN {
			return nil, nil
		}
		return nil, err
	}
	return buf[:n], nil
}

// Close посылает команду в ядро Linux на остановку генерации потока (STREAMOFF) и закрывает файл устройства.
func (c *LinuxCamera) Close() error {
	if c.file != nil {
		var bufType uint32 = v4l2BufferTypeVideoCapture
		// Останавливаем стриминг на уровне драйвера
		syscall.Syscall(syscall.SYS_IOCTL, c.file.Fd(), vidiocStreamOff, uintptr(unsafe.Pointer(&bufType)))
		return c.file.Close()
	}
	return nil
}
