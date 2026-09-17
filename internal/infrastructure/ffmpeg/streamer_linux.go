//go:build linux

package ffmpeg

import (
	"context"
	"fmt"
	"io"
	"log"
	"os/exec"
	"syscall"
)

type FFmpegStreamer struct{}

func NewStreamer() *FFmpegStreamer {
	return &FFmpegStreamer{}
}

func (s *FFmpegStreamer) Start(ctx context.Context, url string, width, height, fps int) (<-chan []byte, <-chan error, error) {
	// Формируем аргументы для нарезки потока на JPEG-кадры в stdout
	args := []string{
		"-rtsp_transport", "tcp",
		"-i", url,
		"-vf", fmt.Sprintf("fps=%d,scale=%d:%d", fps, width, height),
		"-f", "image2pipe",
		"-vcodec", "mjpeg",
		"pipe:1", // вывод в stdout
	}

	cmd := exec.CommandContext(ctx, "ffmpeg", args...)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

	// Получаем пайп для чтения данных из stdout FFmpeg
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create stdout pipe: %w", err)
	}
	cmd.Stderr = log.Writer()

	if err := cmd.Start(); err != nil {
		return nil, nil, fmt.Errorf("failed to start ffmpeg: %w", err)
	}

	videoChan := make(chan []byte, 10)
	errChan := make(chan error, 1)

	// Горутина для чтения кадров
	go func() {
		defer close(videoChan)
		defer close(errChan)

		// Буфер для чтения кусков данных. JPEG начинается с 0xFF 0xD8 и заканчивается 0xFF 0xD9.
		// Для простоты и высокой производительности используем чтение блоками.
		// Если у вас в оригинальном коде был кастомный сплиттер кадров, подставьте его логику сюда.
		buf := make([]byte, 64*1024) 
		for {
			n, err := stdout.Read(buf)
			if n > 0 {
				// Копируем прочитанный кусок, чтобы не было гонки данных
				chunk := make([]byte, n)
				copy(chunk, buf[:n])
				
				select {
				case videoChan <- chunk:
				case <-ctx.Done():
					return
				}
			}
			if err != nil {
				if err != io.EOF && ctx.Err() == nil {
					errChan <- err
				}
				return
			}
		}
	}()

	// Горутина для контроля жизненного цикла процесса
	go func() {
		err := cmd.Wait()
		if err != nil && ctx.Err() == nil {
			log.Printf("FFmpeg process exited with error: %v", err)
		}
		
		// Если контекст отменен, принудительно гасим группу процессов
		if ctx.Err() != nil {
			_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
		}
	}()

	return videoChan, errChan, nil
}
