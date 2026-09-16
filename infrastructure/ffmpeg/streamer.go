package ffmpeg

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os/exec"
	"runtime"
	"strconv"
)

type FFmpegStreamer struct{}

func NewFFmpegStreamer() *FFmpegStreamer {
	return &FFmpegStreamer{}
}

func (s *FFmpegStreamer) Start(ctx context.Context, path string, width, height, fps int) (<-chan []byte, <-chan error, error) {
	var args []string

	// Кроссплатформенная настройка драйверов ввода FFmpeg
	switch runtime.GOOS {
	case "windows":
		args = []string{"-f", "dshow"} // DirectShow для Windows
	case "darwin":
		args = []string{"-f", "avfoundation"} // AVFoundation для macOS
	default:
		args = []string{"-f", "v4l2"} // Video4Linux2 для Linux
	}

	// Добавляем настройки разрешения и FPS
	args = append(args,
		"-video_size", fmt.Sprintf("%dx%d", width, height),
		"-framerate", strconv.Itoa(fps),
		"-i", path, // Путь к устройству
		"-c:v", "mjpeg", // Кодируем в MJPEG
		"-f", "mpjpeg", // Формат потока — многокомпонентный JPEG
		"-an", // Отключаем аудио-трек
		"pipe:1", // Гоним поток в stdout (пайп)
	)

	// Создаем команду запуска процесса ffmpeg с контекстом для автоматического завершения
	cmd := exec.CommandContext(ctx, "ffmpeg", args...)

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create stdout pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return nil, nil, fmt.Errorf("failed to start ffmpeg: %w", err)
	}

	frameChan := make(chan []byte, 5)
	errChan := make(chan error, 1)

	// Читатель потока FFmpeg
	go func() {
		defer func() {
			_ = cmd.Process.Kill() // Гарантированно убиваем процесс при выходе
			_ = cmd.Wait()
			close(frameChan)
			close(errChan)
		}()

		// Инициализируем буферизированный читатель.
		// MJPEG использует маркеры начала кадра (0xFFD8) и конца кадра (0xFFD9).
		// Однако утилита ffmpeg при формате mpjpeg сама расставляет boundary,
		// но проще читать поток блоками через Split-функцию или кастомный парсер партиций.

		// Для простоты примера используем чтение кусками (в реальном продакшене
		// используется парсер multipart-потока по байтовым маркерам JPEG)
		reader := bufio.NewReader(stdout)

		for {
			select {
			case <-ctx.Done():
				return
			default:
				// Реализация чтения отдельного кадра по маркерам JPEG:
				// 1. Ищем маркер 0xFFD8 (Start of Image)
				// 2. Читаем до маркера 0xFFD9 (End of Image)
				frame, err := readJPEGFrame(reader)
				if err != nil {
					if err != io.EOF {
						errChan <- err
					}
					return
				}

				select {
				case frameChan <- frame:
				case <-ctx.Done():
					return
				}
			}
		}
	}()

	return frameChan, errChan, nil
}

// Простейший низкоуровневый парсер JPEG-кадров из бинарного стрима
func readJPEGFrame(r *bufio.Reader) ([]byte, error) {
	var frame []byte

	// Синхронизация: ищем начало JPEG (0xFF, 0xD8)
	for {
		b, err := r.ReadByte()
		if err != nil {
			return nil, err
		}
		if b == 0xFF {
			next, err := r.ReadByte()
			if err != nil {
				return nil, err
			}
			if next == 0xD8 {
				frame = append(frame, 0xFF, 0xD8)
				break
			}
		}
	}

	// Читаем тело кадра, пока не встретим конец JPEG (0xFF, 0xD9)
	for {
		b, err := r.ReadByte()
		if err != nil {
			return nil, err
		}
		frame = append(frame, b)

		if b == 0xFF {
			next, err := r.ReadByte()
			if err != nil {
				return nil, err
			}
			frame = append(frame, next)
			if next == 0xD9 {
				break // Кадр успешно прочитан полностью
			}
		}
	}

	return frame, nil
}
