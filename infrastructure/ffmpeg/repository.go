package ffmpeg

import (
	"bufio"
	"context"
	"fmt"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"webcam-streamer/domain"
)

type FFmpegRepository struct{}

func NewFFmpegRepository() *FFmpegRepository {
	return &FFmpegRepository{}
}

// List возвращает список доступных камер в зависимости от текущей ОС.
func (r *FFmpegRepository) List(ctx context.Context) ([]domain.Camera, error) {
	if runtime.GOOS == "windows" {
		return r.listWindowsDevices(ctx)
	}
	return r.listLinuxDevices(ctx)
}

// listLinuxDevices сканирует систему без тяжелых внешних Си-зависимостей.
func (r *FFmpegRepository) listLinuxDevices(ctx context.Context) ([]domain.Camera, error) {
	matches, err := filepath.Glob("/dev/video*")
	if err != nil {
		return nil, fmt.Errorf("failed to scan linux video devices: %w", err)
	}

	var cameras []domain.Camera
	for _, match := range matches {
		id := strings.TrimPrefix(match, "/dev/")
		cameras = append(cameras, domain.Camera{
			ID:   id,
			Path: match,
			Name: "Linux USB Camera " + id,
		})
	}
	return cameras, nil
}

// listWindowsDevices автоматически парсит устройства через FFmpeg CLI.
func (r *FFmpegRepository) listWindowsDevices(ctx context.Context) ([]domain.Camera, error) {
	ffmpegPath := "ffmpeg"
	if runtime.GOOS == "windows" {
		ffmpegPath = "C:\\ffmpeg\\bin\\ffmpeg.exe"
	}

	cmd := exec.CommandContext(ctx, ffmpegPath, "-list_devices", "true", "-f", "dshow", "-i", "dummy")
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return nil, fmt.Errorf("failed to create stderr pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("failed to start ffmpeg: %w", err)
	}

	var cameras []domain.Camera
	scanner := bufio.NewScanner(stderr)

	// Ищем строки вида: [dshow ...]  "Integrated Camera" (video)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.Contains(strings.ToLower(line), "(video)") {
			start := strings.Index(line, "\"")
			end := strings.LastIndex(line, "\"")
			if start != -1 && end != -1 && start < end {
				deviceName := line[start+1 : end]
				cameras = append(cameras, domain.Camera{
					ID:   deviceName,
					Path: "video=" + deviceName,
					Name: deviceName,
				})
			}
		}
	}

	_ = scanner.Err()
	_ = cmd.Wait()

	// Если камер нет, отдаем заглушку, чтобы интерфейс не был пустым
	if len(cameras) == 0 {
		return []domain.Camera{
			{ID: "cam0", Path: "video=Integrated Camera", Name: "Default Windows Camera (Placeholder)"},
		}, nil
	}

	return cameras, nil
}
