//go:build linux

package camera

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/jpeg"
	"os"
	"strings"

	"github.com/blackjack/webcam"
	"webcam-streamer/internal/domain"
)

// LinuxScanner реализует интерфейс domain.CameraScanner для операционной системы Linux.
type LinuxScanner struct{}

type LinuxCamera struct {
	cam    *webcam.Webcam
	width  int
	height int
}

func NewCamera() domain.VideoCapture {
	return &LinuxCamera{}
}

// NewScanner инициализирует и возвращает Linux-реализацию интерфейса сканирования устройств.
func NewScanner() domain.CameraScanner {
	return &LinuxScanner{}
}

// Scan считывает системную директорию /dev и sysfs в Linux для поиска всех подключенных USB-камер.
// Включает универсальную фильтрацию: отсекает ноды метаданных, не поддерживающие видеозахват.
func (s *LinuxScanner) Scan() ([]domain.DeviceInfo, error) {
	var devices []domain.DeviceInfo
	files, err := os.ReadDir("/dev")
	if err != nil {
		return nil, fmt.Errorf("не удалось прочитать директорию /dev: %w", err)
	}

	for _, file := range files {
		name := file.Name()
		if strings.HasPrefix(name, "video") && !strings.Contains(name, "-") {
			devPath := "/dev/" + name

			// Пытаемся сделать тестовое открытие ноды через blackjack/webcam.
			// Если это нода метаданных, библиотека вернет ошибку ioctl, и мы её пропустим.
			testCam, err := webcam.Open(devPath)
			if err != nil {
				// Нода не поддерживает видеопоток capture, игнорируем её
				continue
			}
			testCam.Close() // Обязательно закрываем дескриптор сразу после теста!

			friendlyName := "Универсальная USB-камера (" + name + ")"
			sysNamePath := fmt.Sprintf("/sys/class/video4linux/%s/name", name)

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

// Init открывает камеру и настраивает формат YUYV через официальную библиотеку blackjack/webcam
func (c *LinuxCamera) Init(path string) error {
	// Создаем список нод для проверки. Если упала /dev/video0, приложение автоматически проверит /dev/video1
	nodesToTry := []string{path}
	if path == "/dev/video0" {
		nodesToTry = append(nodesToTry, "/dev/video1")
	} else if path == "/dev/video1" {
		nodesToTry = append(nodesToTry, "/dev/video0")
	}

	var cam *webcam.Webcam
	var err error
	var activePath string

	// Автоматический перебор нод
	for _, node := range nodesToTry {
		fmt.Printf("[INIT] Открытие камеры %s через blackjack/webcam...\n", node)
		cam, err = webcam.Open(node)
		if err == nil {
			activePath = node
			break
		}
		fmt.Printf("[WARN] Не удалось открыть ноду %s: %v\n", node, err)
	}

	if cam == nil {
		return fmt.Errorf("не удалось инициализировать ни одну из нод камер %v", nodesToTry)
	}

	c.cam = cam
	c.width = 640
	c.height = 480

	// Устанавливаем формат YUYV (FourCC код для YUYV в библиотеке blackjack/webcam)
	// Функция сама под капотом выполнит правильный ioctl с нужным выравниванием памяти!
	pixelFormat := webcam.PixelFormat(uint32('Y') | uint32('U')<<8 | uint32('Y')<<16 | uint32('V')<<24)
	_, _, _, err = c.cam.SetImageFormat(pixelFormat, uint32(c.width), uint32(c.height))
	if err != nil {
		c.Close()
		return fmt.Errorf("драйвер камеры отклонил формат YUYV: %w", err)
	}

	// Запускаем трансляцию потока (STREAMON)
	err = c.cam.StartStreaming()
	if err != nil {
		c.Close()
		return fmt.Errorf("ошибка запуска потока StartStreaming: %w", err)
	}

	fmt.Printf("[SUCCESS] Драйвер blackjack/webcam успешно запустил камеру на %s!\n", activePath)
	return nil
}

// ReadFrame забирает готовый кадр из библиотеки, конвертирует YUYV в JPEG и возвращает буфер
func (c *LinuxCamera) ReadFrame() ([]byte, error) {
	// Предотвращаем панику рантайма (SIGSEGV), если структура закрывается параллельно
	if c == nil || c.cam == nil {
		return nil, fmt.Errorf("устройство камеры не инициализировано или закрыто")
	}

	// Ожидаем готовности кадра от ядра (блокирующий вызов)
	err := c.cam.WaitForFrame(1) // таймаут 1 секунда
	if err != nil {
		return nil, nil // аналог EAGAIN (кадр еще не готов)
	}

	// Повторная атомарная проверка на случай, если за 1 секунду ожидания камеру переключили
	if c.cam == nil {
		return nil, fmt.Errorf("устройство камеры было принудительно закрыто во время ожидания")
	}

	// Читаем сырые байты YUYV из памяти ядра
	rawYuyv, err := c.cam.ReadFrame()
	if err != nil {
		return nil, fmt.Errorf("ошибка чтения кадра ReadFrame: %w", err)
	}

	if len(rawYuyv) == 0 {
		return nil, nil
	}

	// Конвертируем сырой YUYV поток в сжатый JPEG на лету
	localYuyv := make([]byte, len(rawYuyv))
	copy(localYuyv, rawYuyv)

	// Вызываем метод возврата кадра в очередь драйвера Linux
	if r, ok := interface{}(c.cam).(interface{ ReleaseFrame() }); ok {
		r.ReleaseFrame()
	} else if r, ok := interface{}(c.cam).(interface{ Release() }); ok {
		r.Release()
	}

	jpegBytes, err := convertYuyvToJpeg(localYuyv, c.width, c.height)
	if err != nil {
		return nil, fmt.Errorf("ошибка конвертации кадра YUYV->JPEG: %w", err)
	}

	return jpegBytes, nil
}

// Close корректно останавливает стрим и освобождает память
func (c *LinuxCamera) Close() error {
	if c.cam == nil {
		return nil
	}
	_ = c.cam.StopStreaming()
	err := c.cam.Close()
	c.cam = nil
	return err
}

// Вспомогательная функция распаковки YUYV макропикселей в RGBA и сжатия в JPEG
func convertYuyvToJpeg(yuyv []byte, width, height int) ([]byte, error) {
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	bounds := len(yuyv) - 3
	idx := 0

	for i := 0; i < bounds && idx < width*height; i += 4 {
		y0 := float64(yuyv[i])
		u := float64(yuyv[i+1]) - 128
		y1 := float64(yuyv[i+2]) - 128
		v := float64(yuyv[i+3]) - 128

		r0 := y0 + 1.402*v
		g0 := y0 - 0.344136*u - 0.714136*v
		b0 := y0 + 1.772*u

		r1 := y1 + 1.402*v
		g1 := y1 - 0.344136*u - 0.714136*v
		b1 := y1 + 1.772*u

		x0 := idx % width
		ptY0 := idx / width
		img.Set(x0, ptY0, color.RGBA{uint8(clamp(r0)), uint8(clamp(g0)), uint8(clamp(b0)), 255})
		idx++

		x1 := idx % width
		ptY1 := idx / width
		img.Set(x1, ptY1, color.RGBA{uint8(clamp(r1)), uint8(clamp(g1)), uint8(clamp(b1)), 255})
		idx++
	}

	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: 80}); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func clamp(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 255 {
		return 255
	}
	return v
}
