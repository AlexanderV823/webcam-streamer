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
	"unsafe"

	"golang.org/x/sys/unix"
	"webcam-streamer/internal/domain"
)

const (
	// v4l2BufferTypeVideoCapture указывает ядру, что буфер используется для захвата видеопотока
	v4l2BufferTypeVideoCapture = 1
	// v4l2MemoryMmap задает режим потокового обмена через проецирование памяти ядра (Memory Mapping)
	v4l2MemoryMmap             = 1
	// vidiocSFmt (Set Format) устанавливает геометрию кадра (разрешение) и кодек (FourCC код) в драйвере
	vidiocSFmt      = 0xc0cc5605
	// vidiocReqBufs (Request Buffers) запрашивает у ядра выделение определенного количества буферов под кадры
	vidiocReqBufs   = 0xc0145608
	// vidiocQueryBuf запрашивает параметры буфера (размер и смещение в памяти ядра) для последующего mmap
	vidiocQueryBuf  = 0xc0445609
	// vidiocQBuf (Queue Buffer) отправляет пустой буфер в очередь ядра, разрешая камере записывать туда новый кадр
	vidiocQBuf      = 0xc044560f
	// vidiocDQBuf (Dequeue Buffer) извлекает из очереди ядра буфер, который уже заполнен свежими данными кадра
	vidiocDQBuf     = 0xc0445611
	// vidiocStreamOn запускает генерацию видеопотока и захват кадров на физическом сенсоре камеры
	vidiocStreamOn  = 0x4004564a
	// vidiocStreamOff останавливает генерацию видеопотока на камере
	vidiocStreamOff = 0x4004564b
	// Указываем ядру режим обмена через Read/Write дескрипторы
	v4l2MemoryReadwrite = 1
)

// LinuxScanner реализует интерфейс domain.CameraScanner для операционной системы Linux.
type LinuxScanner struct{}

// LocalBuffer хранит ссылку на область памяти, спроецированную из ядра
type LocalBuffer struct {
	Slice []byte
}

type LinuxCamera struct {
	file    *os.File
	buffers []LocalBuffer
	width   int
	height  int
}

// Структура v4l2_requestbuffers для запроса буферов у ядра
type v4l2RequestBuffers struct {
	Count    uint32
	Type     uint32
	Memory   uint32
	Reserved [2]uint32
}

// Структура v4l2_timecode для использования внутри v4l2_buffer
type v4l2Timecode struct {
	Type     uint32
	Flags    uint32
	Frames   uint8
	Seconds  uint8
	Minutes  uint8
	Hours    uint8
	Userbits [4]uint8
}

// Структура v4l2_buffer для постановки/снятия кадров из очереди (QBUF/DQBUF)
type v4l2Buffer struct {
	Index     uint32
	Type      uint32
	BytesUsed uint32
	Flags     uint32
	Field     uint32
	Timestamp unix.Timeval
	Timecode  v4l2Timecode
	Sequence  uint32
	Memory    uint32
	Offset    uint32 // Union: в режиме MMAP здесь лежит смещение буфера
	Length    uint32
	Reserved2 uint32
	Reserved  uint32
}

type v4l2PixFormat struct {
	Width        uint32
	Height       uint32
	Pixelformat  uint32
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
	Type    uint32
	RawData [204]byte
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
	files, err := os.ReadDir("/dev")
	if err != nil {
		return nil, fmt.Errorf("не удалось прочитать директорию /dev: %w", err)
	}

	for _, file := range files {
		name := file.Name()
		if strings.HasPrefix(name, "video") && !strings.Contains(name, "-") {
			devPath := "/dev/" + name
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

// Init открывает устройство, настраивает формат, запрашивает MMAP буферы и запускает стрим
func (c *LinuxCamera) Init(path string) error {
	fd, err := unix.Open(path, unix.O_RDWR|unix.O_NONBLOCK, 0)
	if err != nil {
		return fmt.Errorf("не удалось открыть устройство камеры %s: %w", path, err)
	}
	c.file = os.NewFile(uintptr(fd), path)

	// Меняем FourCC код с MJPG на YUYV (0x56595559)
	var yuyvFourCC uint32 = uint32('Y') | uint32('U')<<8 | uint32('Y')<<16 | uint32('V')<<24
	var f v4l2Format
	f.Type = v4l2BufferTypeVideoCapture

	c.width = 640
	c.height = 480

	pixFmt := (*v4l2PixFormat)(unsafe.Pointer(&f.RawData[0]))
	pixFmt.Width = uint32(c.width)
	pixFmt.Height = uint32(c.height)
	pixFmt.Pixelformat = yuyvFourCC
	pixFmt.Field = 1 // V4L2_FIELD_NONE

	_, _, sysErr := unix.Syscall(unix.SYS_IOCTL, c.file.Fd(), vidiocSFmt, uintptr(unsafe.Pointer(&f)))
	if sysErr != 0 {
		return fmt.Errorf("драйвер камеры отклонил формат YUYV: %v", sysErr)
	}

	// 2. Запрос буферов (REQBUFS) у ядра Linux (запрашиваем 4 буфера для плавности)
	var reqBufs v4l2RequestBuffers
	reqBufs.Count = 4
	reqBufs.Type = v4l2BufferTypeVideoCapture
	reqBufs.Memory = v4l2MemoryMmap

	_, _, sysErr = unix.Syscall(unix.SYS_IOCTL, c.file.Fd(), vidiocReqBufs, uintptr(unsafe.Pointer(&reqBufs)))
	if sysErr != 0 {
		c.Close()
		return fmt.Errorf("ошибка ioctl VIDIOC_REQBUFS: %v", sysErr)
	}

	c.buffers = make([]LocalBuffer, reqBufs.Count)

	// 3. Проекция памяти ядра в Go (QUERYBUF + MMAP) и заполнение начальной очереди
	for i := uint32(0); i < reqBufs.Count; i++ {
		var buf v4l2Buffer
		buf.Index = i
		buf.Type = v4l2BufferTypeVideoCapture
		buf.Memory = v4l2MemoryMmap

		_, _, sysErr = unix.Syscall(unix.SYS_IOCTL, c.file.Fd(), vidiocQueryBuf, uintptr(unsafe.Pointer(&buf)))
		if sysErr != 0 {
			c.Close()
			return fmt.Errorf("ошибка ioctl VIDIOC_QUERYBUF для буфера %d: %v", i, sysErr)
		}

		// Вызываем системный mmap
		mmapSlice, err := unix.Mmap(int(c.file.Fd()), int64(buf.Offset), int(buf.Length), unix.PROT_READ|unix.PROT_WRITE, unix.MAP_SHARED)
		if err != nil {
			c.Close()
			return fmt.Errorf("ошибка mmap для буфера %d: %w", i, err)
		}
		c.buffers[i] = LocalBuffer{Slice: mmapSlice}

		// Сразу же отправляем пустой буфер в очередь ядра, чтобы оно могло начать его заполнять
		_, _, sysErr = unix.Syscall(unix.SYS_IOCTL, c.file.Fd(), vidiocQBuf, uintptr(unsafe.Pointer(&buf)))
		if sysErr != 0 {
			c.Close()
			return fmt.Errorf("ошибка ioctl VIDIOC_QBUF для буфера %d: %v", i, sysErr)
		}
	}

	// 4. Запуск трансляции в ядре (STREAMON)
	var bufType uint32 = v4l2BufferTypeVideoCapture
	_, _, sysErr = unix.Syscall(unix.SYS_IOCTL, c.file.Fd(), vidiocStreamOn, uintptr(unsafe.Pointer(&bufType)))
	if sysErr != 0 && sysErr != unix.EBUSY {
		c.Close()
		return fmt.Errorf("ошибка ioctl VIDIOC_STREAMON: %v", sysErr)
	}

	fmt.Println("[SUCCESS] Драйвер V4L2 MMAP (YUYV) успешно инициализирован и запущен!")
	return nil
}

// ReadFrame забирает готовый кадр из очереди ядра, конвертирует YUYV в JPEG и возвращает буфер обратно
func (c *LinuxCamera) ReadFrame() ([]byte, error) {
	if c.file == nil || len(c.buffers) == 0 {
		return nil, fmt.Errorf("камера не инициализирована")
	}

	// 1. Извлекаем заполненный буфер из очереди ядра (DQBUF)
	var buf v4l2Buffer
	buf.Type = v4l2BufferTypeVideoCapture
	buf.Memory = v4l2MemoryMmap

	_, _, sysErr := unix.Syscall(unix.SYS_IOCTL, c.file.Fd(), vidiocDQBuf, uintptr(unsafe.Pointer(&buf)))
	if sysErr != 0 {
		// Если кадр еще не подготовлен сенсором камеры (EAGAIN), возвращаем пустой результат без ошибки
		if sysErr == unix.EAGAIN {
			return nil, nil
		}
		return nil, fmt.Errorf("ошибка ioctl VIDIOC_DQBUF: %v", sysErr)
	}

	// Безопасно извлекаем индекс буфера, который заполнило ядро
	if buf.Index >= uint32(len(c.buffers)) {
		return nil, fmt.Errorf("некорректный индекс буфера от ядра: %d", buf.Index)
	}

	// Извлекаем сырые YUYV байты
	rawYuyv := c.buffers[buf.Index].Slice[:buf.BytesUsed]

	// Конвертируем сырой YUYV поток в сжатый JPEG на лету
	jpegBytes, err := convertYuyvToJpeg(rawYuyv, c.width, c.height)
	if err != nil {
		// Возвращаем буфер обратно ядру даже в случае ошибки конвертации (исправлен возврат до 3 значений)
		_, _, _ = unix.Syscall(unix.SYS_IOCTL, c.file.Fd(), vidiocQBuf, uintptr(unsafe.Pointer(&buf)))
		return nil, fmt.Errorf("ошибка конвертации кадра YUYV->JPEG: %w", err)
	}

	// Возвращаем буфер обратно в очередь ядра (исправлено с 2 переменных до 3)
	_, _, sysErr = unix.Syscall(unix.SYS_IOCTL, c.file.Fd(), vidiocQBuf, uintptr(unsafe.Pointer(&buf)))
	if sysErr != 0 {
		return nil, fmt.Errorf("ошибка ioctl VIDIOC_QBUF при возврате буфера: %v", sysErr)
	}

	return jpegBytes, nil
}

// Close останавливает поток в ядре, делает Munmap для всех буферов и закрывает дескриптор файла
func (c *LinuxCamera) Close() error {
	if c.file == nil {
		return nil
	}

	// 1. Останавливаем поток в ядре (STREAMOFF)
	var bufType uint32 = v4l2BufferTypeVideoCapture
	_, _, _ = unix.Syscall(unix.SYS_IOCTL, c.file.Fd(), vidiocStreamOff, uintptr(unsafe.Pointer(&bufType)))

	// 2. Освобождаем промаппированную память (Munmap)
	for _, buf := range c.buffers {
		if buf.Slice != nil {
			_ = unix.Munmap(buf.Slice)
		}
	}
	c.buffers = nil

	// 3. Закрываем файл устройства
	err := c.file.Close()
	c.file = nil
	return err
}

// convertYuyvToJpeg распаковывает YUYV макропикселей в RGB и сжатия в JPEG
func convertYuyvToJpeg(yuyv []byte, width, height int) ([]byte, error) {
	img := image.NewRGBA(image.Rect(0, 0, width, height))

	// В YUYV каждые 4 байта кодируют 2 пикселя: [Y0, U, Y1, V]
	// Y0 - яркость пикселя 1, Y1 - яркость пикселя 2. U и V - общие цветовые компоненты
	bounds := len(yuyv) - 3
	idx := 0

	for i := 0; i < bounds && idx < width*height; i += 4 {
		y0 := float64(yuyv[i])
		u  := float64(yuyv[i+1]) - 128
		y1 := float64(yuyv[i+2]) - 128
		v  := float64(yuyv[i+3]) - 128

		// Пиксель 1
		r0 := y0 + 1.402*v
		g0 := y0 - 0.344136*u - 0.714136*v
		b0 := y0 + 1.772*u

		// Пиксель 2
		r1 := y1 + 1.402*v
		g1 := y1 - 0.344136*u - 0.714136*v
		b1 := y1 + 1.772*u

		// Записываем Пиксель 1 в сетку RGBA
		x0 := idx % width
		ptY0 := idx / width
		img.Set(x0, ptY0, color.RGBA{uint8(clamp(r0)), uint8(clamp(g0)), uint8(clamp(b0)), 255})
		idx++

		// Записываем Пиксель 2 в сетку RGBA
		x1 := idx % width
		ptY1 := idx / width
		img.Set(x1, ptY1, color.RGBA{uint8(clamp(r1)), uint8(clamp(g1)), uint8(clamp(b1)), 255})
		idx++
	}

	var buf bytes.Buffer
	// Сжимаем RGBA в JPEG с качеством 80% (оптимально для баланса нагрузка/качество)
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: 80}); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func clamp(v float64) float64 {
	if v < 0 { return 0 }
	if v > 255 { return 255 }
	return v
}