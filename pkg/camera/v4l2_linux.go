//go:build linux

package camera

import (
	"fmt"
	"os"
	"strings"
	"unsafe"

	"golang.org/x/sys/unix"
	"webcam-streamer/internal/domain"
)

const (
	// v4l2BufferTypeVideoCapture указывает ядру Linux, что буфер используется для захвата видео
	v4l2BufferTypeVideoCapture = 1
	// vidiocStreamOn — код системного вызова ioctl для запуска трансляции с камеры
	vidiocStreamOn = 0x4004564a
	// vidiocStreamOff — код системного вызова ioctl для остановки трансляции с камеры
	vidiocStreamOff = 0x4004564b
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
	// Открываем устройство через unix-пакет с флагами чтения-записи и неблокирующего режима
	fd, err := unix.Open(path, unix.O_RDWR|unix.O_NONBLOCK, 0)
	if err != nil {
		return fmt.Errorf("не удалось открыть устройство камеры %s: %w", path, err)
	}

	// Оборачиваем системный дескриптор в стандартный файл Go, чтобы использовать методы Read/Close
	c.file = os.NewFile(uintptr(fd), path)

	var bufType uint32 = v4l2BufferTypeVideoCapture

	// Выполняем системный вызов ioctl напрямую через пакет unix
	err = unix.IoctlSetInt(int(c.file.Fd()), vidiocStreamOn, int(uintptr(unsafe.Pointer(&bufType))))
	if err != nil && err != unix.EBUSY {
		c.file.Close()
		return fmt.Errorf("ошибка ioctl VIDIOC_STREAMON: %w", err)
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
		// Проверяем ошибку на соответствие unix.EAGAIN (ресурс временно недоступен / кадр не готов)
		if perr, ok := err.(*os.PathError); ok && perr.Err == unix.EAGAIN {
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
		// Завершаем стрим в ядре перед закрытием файла
		unix.IoctlSetInt(int(c.file.Fd()), vidiocStreamOff, int(uintptr(unsafe.Pointer(&bufType))))
		return c.file.Close()
	}
	return nil
}
