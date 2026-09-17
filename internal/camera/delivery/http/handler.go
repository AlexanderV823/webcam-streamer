package http

import (
	"context"
	"encoding/json"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"strconv"
	"webcam-streamer/internal/camera/usecase"
)

// HTTPHandler управляет обработкой всех сетевых эндпоинтов приложения.
type HTTPHandler struct {
	uc *usecase.CameraUseCase
}

// NewHTTPHandler создает новый экземпляр HTTPHandler с внедренным сценарием использования.
func NewHTTPHandler(uc *usecase.CameraUseCase) *HTTPHandler {
	return &HTTPHandler{uc: uc}
}

// RegisterRoutes связывает эндпоинты с методами обработки и оборачивает роутер в middleware логирования.
func (h *HTTPHandler) RegisterRoutes(mux *http.ServeMux) http.Handler {
	mux.HandleFunc("/", h.HandleIndex)
	mux.HandleFunc("/api/cameras", h.HandleCameras)
	mux.HandleFunc("/stream", h.HandleStream)

	return LoggingMiddleware(mux)
}

// HandleIndex отдает клиенту заглавную HTML-страницу интерфейса управления.
func (h *HTTPHandler) HandleIndex(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	fmt.Fprint(w, HTMLPage)
}

// HandleCameras возвращает список доступных видеоустройств сервера в формате JSON.
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

// HandleStream организует бесконечную многокомпонентную MJPEG-трансляцию кадров,
// динамически подстраиваясь под переданные параметры w (ширина), h (высота) и fps.
func (h *HTTPHandler) HandleStream(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Query().Get("dev")
	if path == "" {
		http.Error(w, "Missing 'dev' parameter", http.StatusBadRequest)
		return
	}

	// Парсинг параметров качества видео с дефолтными значениями
	width := getQueryInt(r, "w", 640)
	height := getQueryInt(r, "h", 480)
	fps := getQueryInt(r, "fps", 30)

	// Привязываем жизненный цикл стрима к контексту HTTP-запроса клиента
	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()

	frameChan, errChan, err := h.uc.GetStream(ctx, path, width, height, fps)
	if err != nil {
		http.Error(w, "Failed to initialize stream: "+err.Error(), http.StatusInternalServerError)
		return
	}

	mimeWriter := multipart.NewWriter(w)
	w.Header().Set("Content-Type", "multipart/x-mixed-replace; boundary="+mimeWriter.Boundary())
	w.WriteHeader(http.StatusOK)

	for {
		select {
		case <-ctx.Done():
			// Клиент отключился (закрыл вкладку или прервал соединение)
			return

		case err, ok := <-errChan:
			if !ok {
				// Инфраструктурный слой закрыл канал ошибок — стрим завершен
				return
			}
			if err != nil {
				// Произошла реальная ошибка чтения с физической камеры
				return
			}

		case frame, ok := <-frameChan:
			if !ok {
				// Канал кадров закрыт (например, камера была отключена физически)
				return
			}

			// Формируем заголовки для текущего JPEG-кадра в MJPEG-потоке
			partHeader := make(textproto.MIMEHeader)
			partHeader.Set("Content-Type", "image/jpeg")
			partHeader.Set("Content-Length", fmt.Sprintf("%d", len(frame)))

			partWriter, err := mimeWriter.CreatePart(partHeader)
			if err != nil {
				return
			}

			// Записываем бинарные данные кадра в HTTP-ответ
			if _, err := partWriter.Write(frame); err != nil {
				// Ошибка записи (клиент мог оборвать соединение во время отправки)
				return
			}
		}
	}
}

// getQueryInt извлекает из URL именованный целочисленный параметр,
// возвращая дефолтное значение в случае отсутствия параметра или ошибки парсинга.
func getQueryInt(r *http.Request, key string, defaultVal int) int {
	valStr := r.URL.Query().Get(key)
	if valStr == "" {
		return defaultVal
	}
	val, err := strconv.Atoi(valStr)
	if err != nil {
		return defaultVal
	}
	return val
}
