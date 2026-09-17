package ffmpeg

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
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

	ffmpegPath := "ffmpeg"
	if runtime.GOOS == "windows" {
		ffmpegPath = "C:\\ffmpeg\\bin\\ffmpeg.exe"
	}

	// Создаем команду запуска процесса ffmpeg с контекстом для автоматического завершения
	cmd := exec.CommandContext(ctx, ffmpegPath, args...)

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		log.Printf("❌ FFmpeg Streamer Error: не удалось создать stdout pipe: %v", err)
		return nil, nil, fmt.Errorf("failed to create stdout pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		log.Printf("❌ FFmpeg Streamer Error: не удалось запустить ffmpeg по пути %s. Ошибка: %v", ffmpegPath, err)
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

		reader := bufio.NewReader(stdout)

		for {
			select {
			case <-ctx.Done():
				return
			default:
				// Читаем отдельный кадр по исправленному алгоритму с Peek
				frame, err := readJPEGFrame(reader)
				if err != nil {
					if err != io.EOF && !errors.Is(err, os.ErrClosed) {
						log.Printf("⚠️ FFmpeg Streamer Warning: ошибка чтения JPEG кадра: %v", err)
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

// Исправленный надежный парсер JPEG-кадров из бинарного стрима FFmpeg
func readJPEGFrame(r *bufio.Reader) ([]byte, error) {
	var frame []byte

	// 1. Синхронизация: ищем начало JPEG (0xFF, 0xD8)
	for {
		b, err := r.ReadByte()
		if err != nil {
			return nil, err
		}
		if b == 0xFF {
			// Проверяем следующий байт без его удаления из буфера
			nextBytes, err := r.Peek(1)
			if err != nil {
				return nil, err
			}
			if nextBytes[0] == 0xD8 {
				_, _ = r.ReadByte() // Теперь фактически забираем 0xD8 из буфера
				frame = append(frame, 0xFF, 0xD8)
				break
			}
		}
	}

	// 2. Читаем тело кадра, пока не встретим конец JPEG (0xFF, 0xD9)
	for {
		b, err := r.ReadByte()
		if err != nil {
			return nil, err
		}
		frame = append(frame, b)

		if b == 0xFF {
			nextBytes, err := r.Peek(1)
			if err != nil {
				return nil, err
			}
			if nextBytes[0] == 0xD9 {
				_, _ = r.ReadByte() // Забираем 0xD9
				frame = append(frame, 0xD9)
				break // Кадр успешно собран
			}
		}
	}

	return frame, nil
}
