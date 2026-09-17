package http

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"webcam-streamer/internal/camera"
)

type Handler struct {
	useCase camera.CameraUseCase
}

// NewHandler — конструктор для HTTP адаптера
func NewHandler(uc camera.CameraUseCase) *Handler {
	return &Handler{useCase: uc}
}

// StreamHandler обрабатывает запросы на получение MJPEG-потока камеры
func (h *Handler) StreamHandler(w http.ResponseWriter, r *http.Request) {
	cameraID := r.URL.Query().Get("id")
	if cameraID == "" {
		http.Error(w, "Missing 'id' parameter", http.StatusBadRequest)
		return
	}

	// Устанавливаем заголовки для MJPEG стриминга
	w.Header().Set("Content-Type", "multipart/x-mixed-replace; boundary=frame")
	w.Header().Set("Cache-Control", "no-cache, private, no-store, must-revalidate, max-age=0")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Pragma", "no-cache")

	// Создаем контекст, который отменится, если клиент закроет вкладку/разорвет соединение
	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()

	// Запрашиваем стрим у бизнес-логики (параметры: 640x480, 15 FPS)
	videoChan, errChan, err := h.useCase.StartStream(ctx, cameraID, 640, 480, 15)
	if err != nil {
		log.Printf("Failed to start stream for camera %s: %v", cameraID, err)
		http.Error(w, "Failed to initialize camera stream", http.StatusInternalServerError)
		return
	}

	// Потоковая передача кадров клиенту в реальном времени
	for {
		select {
		case <-ctx.Done():
			log.Printf("Client disconnected from stream %s", cameraID)
			return
		case err := <-errChan:
			if err != nil {
				log.Printf("Error during streaming camera %s: %v", cameraID, err)
			}
			return
		case frame, ok := <-videoChan:
			if !ok {
				return
			}
			// Формируем multipart-чанк для MJPEG
			_, err := fmt.Fprintf(w, "--frame\r\nContent-Type: image/jpeg\r\nContent-Length: %d\r\n\r\n", len(frame))
			if err != nil {
				return
			}
			_, err = w.Write(frame)
			if err != nil {
				return
			}
			_, err = w.Write([]byte("\r\n"))
			if err != nil {
				return
			}

			// Сбрасываем буфер в сеть немедленно
			if flusher, ok := w.(http.Flusher); ok {
				flusher.Flush()
			}
		}
	}
}

// ViewHandler отдает веб-интерфейс, вшитый через go:embed
func (h *Handler) ViewHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache, private, no-store")

	// Отдаем массив байт uiHTML, который мы инициализировали в views.go
	_, _ = w.Write(uiHTML)
}
