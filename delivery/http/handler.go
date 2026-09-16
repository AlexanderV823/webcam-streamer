package http

import (
	"context"
	"encoding/json"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"webcam-streamer/usecase"
)

type HTTPHandler struct {
	uc *usecase.CameraUseCase
}

func NewHTTPHandler(uc *usecase.CameraUseCase) *HTTPHandler {
	return &HTTPHandler{uc: uc}
}

// RegisterRoutes настраивает маршруты и возвращает handler с примененными middleware
func (h *HTTPHandler) RegisterRoutes(mux *http.ServeMux) http.Handler {
	mux.HandleFunc("/", h.HandleIndex)
	mux.HandleFunc("/api/cameras", h.HandleCameras)
	mux.HandleFunc("/stream", h.HandleStream)

	// Оборачиваем весь mux в наш logging middleware
	return LoggingMiddleware(mux)
}

func (h *HTTPHandler) HandleIndex(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	fmt.Fprint(w, HTMLPage)
}

func (h *HTTPHandler) HandleCameras(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	
	cameras, err := h.uc.GetAvailableCameras(r.Context())
	if err != nil {
		http.Error(w, `{"error":"failed to get cameras"}`, http.StatusInternalServerError)
		return
	}
	
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(cameras)
}

func (h *HTTPHandler) HandleStream(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Query().Get("dev")
	if path == "" {
		http.Error(w, "Missing 'dev' parameter", http.StatusBadRequest)
		return
	}

	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()

	frameChan, errChan, err := h.uc.GetStream(ctx, path)
	if err != nil {
		http.Error(w, "Failed to initialize stream: "+err.Error(), http.StatusInternalServerError)
		return
	}

	mimeWriter := multipart.NewWriter(w)
	w.Header().Set("Content-Type", "multipart/x-mixed-replace; boundary="+mimeWriter.Boundary())
	
	// Отправляем 200 OK — стрим успешно запущен
	w.WriteHeader(http.StatusOK)

	for {
		select {
		case <-ctx.Done():
			// Если клиент сам закрыл соединение, перехватчик может не зафиксировать кастомный код,
			// но благодаря этому выходу middleware корректно посчитает время удержания стрима.
			return
		case err := <-errChan:
			if err != nil {
				// Если ошибка произошла посреди трансляции, мы просто прерываем цикл.
				// Заголовки уже ушли, поэтому изменить HTTP-статус на 500 нельзя.
				return
			}
		case frame, ok := <-frameChan:
			if !ok {
				return
			}

			partHeader := make(textproto.MIMEHeader)
			partHeader.Set("Content-Type", "image/jpeg")
			partHeader.Set("Content-Length", fmt.Sprintf("%d", len(frame)))

			partWriter, err := mimeWriter.CreatePart(partHeader)
			if err != nil {
				return
			}

			if _, err := partWriter.Write(frame); err != nil {
				return // Клиент отключился в процессе передачи кадра
			}
		}
	}
}
