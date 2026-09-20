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
	// Код для запроса буферов памяти у ядра
	vidiocReqBufs  = 0xc0145608
	// vidiocStreamOn — код системного вызова ioctl для запуска трансляции с камеры
	vidiocStreamOn = 0x4004564a
	// vidiocStreamOff — код системного вызова ioctl для остановки трансляции с камеры
	vidiocStreamOff = 0x4004564b
	// Указываем ядру режим обмена через Read/Write дескрипторы
	v4l2MemoryReadwrite = 1
	// Добавляем код системного вызова для установки формата пикселей
	vidiocSFmt = 0xc0cc5605
)

// LinuxScanner реализует интерфейс domain.CameraScanner для операционной системы Linux.
type LinuxScanner struct{}

// LinuxCamera реализует интерфейс domain.VideoCapture для прямого взаимодействия с V4L2 без CGO.
type LinuxCamera struct {
	file *os.File
}

// Структура v4l2_requestbuffers для системного вызова ioctl
type v4l2RequestBuffers struct {
	Count    uint32 // Количество запрашиваемых буферов кадра (обычно от 1 до 4)
	Type     uint32 // Тип (v4l2BufferTypeVideoCapture)
	Memory   uint32 // Тип памяти (v4l2MemoryReadwrite)
	Reserved [2]uint32
}

// v4l2PixFormat описывает формат пикселей кадра для Linux V4L2
type v4l2PixFormat struct {
	Width        uint32
	Height       uint32
	Pixelformat  uint32 // Сюда мы запишем FourCC код для MJPEG
	Field        uint32
	BytesPerLine uint32
	SizeImage    uint32
	Colorspace   uint32
	Priv         uint32
	Flags        uint32
	Enc          uint32
	Quant        uint32
	XferFunc     uint32
}

// v4l2Format объединяет тип буфера и параметры формата пикселей
type v4l2Format struct {
	Type uint32
	RawData [204]byte // 204 байта под union данных формата (хватает под v4l2_pix_format с запасом)
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

// Init открывает дескриптор файла USB-устройства и подготавливает буферы обмена ядра
func (c *LinuxCamera) Init(path string) error {
	fd, err := unix.Open(path, unix.O_RDWR|unix.O_NONBLOCK, 0)
	if err != nil {
		return fmt.Errorf("не удалось открыть устройство камеры %s: %w", path, err)
	}

	c.file = os.NewFile(uintptr(fd), path)

	// Формируем FourCC код для формата MJPEG (байты 'M', 'J', 'P', 'G')
	var mjpegFourCC uint32 = uint32('M') | uint32('J')<<8 | uint32('P')<<16 | uint32('G')<<24

	// Создаем структуру формата нужного ядру размера
	var f v4l2Format
	f.Type = v4l2BufferTypeVideoCapture

	// Маппим структуру пикселей прямо поверх байтового массива RawData
	pixFmt := (*v4l2PixFormat)(unsafe.Pointer(&f.RawData[0]))
	pixFmt.Width = 640        // Базовое стандартное разрешение
	pixFmt.Height = 480
	pixFmt.Pixelformat = mjpegFourCC
	pixFmt.Field = 1          // V4L2_FIELD_NONE (прогрессивная развертка)

	// Выполняем системный вызов установки формата пикселей
	_, _, sysErr := unix.Syscall(unix.SYS_IOCTL, c.file.Fd(), vidiocSFmt, uintptr(unsafe.Pointer(&f)))
	if sysErr != 0 {
		// Если конкретный драйвер не поддерживает 640x480 MJPEG, логируем системную ошибку ядра
		fmt.Printf("[WARN] Драйвер камеры отклонил формат MJPEG ioctl: %v\n", sysErr)
	} else {
		fmt.Println("[SUCCESS] Драйвер V4L2 успешно переведен в режим захвата MJPEG!")
	}

	// Запуск видеопотока
	var bufType uint32 = v4l2BufferTypeVideoCapture
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
