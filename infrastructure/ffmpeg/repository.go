package ffmpeg

import (
	"fmt"
	"bufio"
	"context"
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

// List возвращает список доступных камер в зависимости от текущей ОС
func (r *FFmpegRepository) List(ctx context.Context) ([]domain.Camera, error) {
	if runtime.GOOS == "windows" {
		return r.listWindowsDevices(ctx)
	}
	return r.listLinuxDevices(ctx)
}

// Сканирование камер для Linux (без внешних Си-библиотек)
func (r *FFmpegRepository) listLinuxDevices(ctx context.Context) ([]domain.Camera, error) {
	matches, err := filepath.Glob("/dev/video*")
	if err != nil {
		return nil, err
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

// Автоматический парсинг подключенных камер в Windows через FFmpeg CLI
func (r *FFmpegRepository) listWindowsDevices(ctx context.Context) ([]domain.Camera, error) {
	// FFmpeg выводит список устройств DirectShow в stderr
	cmd := exec.CommandContext(ctx, "ffmpeg", "-list_devices", "true", "-f", "dshow", "-i", "dummy")

	// Перенаправляем вывод, так как ffmpeg по умолчанию пишет логи в stderr
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return nil, fmt.Errorf("failed to create stderr pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("failed to start ffmpeg command: %w", err)
	}

	var cameras []domain.Camera
	scanner := bufio.NewScanner(stderr)

	// Ищем строки вида: [dshow ...]  "Integrated Camera" (video)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.Contains(line, "(video)") {
			// Извлекаем имя устройства между кавычками
			start := strings.Index(line, "\"")
			end := strings.LastIndex(line, "\"")
			if start != -1 && end != -1 && start < end {
				deviceName := line[start+1 : end]
				cameras = append(cameras, domain.Camera{
					ID:   deviceName,
					Path: "video=" + deviceName, // Формат пути для Windows DirectShow
					Name: deviceName,
				})
			}
		}
	}

	// ИСПРАВЛЕНО: Обязательная проверка на наличие ошибок чтения из потока
	if err := scanner.Err(); err != nil {
		_ = cmd.Process.Kill() // Принудительно завершаем процесс в случае сбоя
		_ = cmd.Wait()
		return nil, fmt.Errorf("error reading ffmpeg output: %w", err)
	}

	_ = cmd.Wait()

	// Если камер нет, отдаем заглушку, чтобы интерфейс не был пустым
	if len(cameras) == 0 {
		return []domain.Camera{
			{ID: "cam0", Path: "video=Integrated Camera", Name: "Default Windows Camera (Placeholder)"},
		}, nil
	}

	return cameras, nil
}
