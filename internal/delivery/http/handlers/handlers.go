// Package handlers содержит контроллеры конечных точек API для обработки веб-запросов.
package handlers

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"webcam-streamer/internal/usecase/auth"
	"webcam-streamer/internal/usecase/stream"
)

// Handlers объединяет обработчики всех API роутов приложения.
type Handlers struct {
	auth   *auth.Usecase
	stream *stream.Usecase
}

// NewHandlers создает новый экземпляр HTTP хэндлеров с необходимыми UseCase зависимостями.
func NewHandlers(au *auth.Usecase, su *stream.Usecase) *Handlers {
	return &Handlers{auth: au, stream: su}
}

type loginReq struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type switchCamReq struct {
	CameraID string `json:"camera_id"`
}

// HandleLogin выполняет аутентификацию администратора по Bcrypt-хэшу и выдает JWT.
func (h *Handlers) HandleLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		h.respondError(w, http.StatusMethodNotAllowed)
		return
	}

	var req loginReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.respondError(w, http.StatusBadRequest)
		return
	}

	token, err := h.auth.Login(req.Username, req.Password)
	if err != nil {
		if errors.Is(err, auth.ErrAuthFailed) {
			h.respondError(w, http.StatusUnauthorized)
			return
		}
		h.respondError(w, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"token": token})
}

// HandleListCameras возвращает список доступных физических устройств
func (h *Handlers) HandleListCameras(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		h.respondError(w, http.StatusMethodNotAllowed)
		return
	}

	// Запрос идет в бизнес-логику, а не напрямую к драйверу железа
	devices, err := h.stream.ListAvailableCameras()
	if err != nil {
		h.respondError(w, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(devices)
}

// HandleSwitchCamera переключает камеру «на лету»
func (h *Handlers) HandleSwitchCamera(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		h.respondError(w, http.StatusMethodNotAllowed)
		return
	}

	var req switchCamReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.respondError(w, http.StatusBadRequest)
		return
	}

	if err := h.stream.SwitchCamera(req.CameraID); err != nil {
		h.respondError(w, http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"status":200,"message":"Камера успешно переключена"}`))
}

// HandleStream организует непрерывную потоковую передачу кадров MJPEG.
func (h *Handlers) HandleStream(w http.ResponseWriter, r *http.Request) {
	frameCh := h.stream.AddListener()
	defer h.stream.RemoveListener(frameCh)

	w.Header().Set("Content-Type", "multipart/x-mixed-replace; boundary=frame")
	w.Header().Set("Cache-Control", "no-cache, private")
	w.Header().Set("Pragma", "no-cache")

	ctx := r.Context()

	for {
		select {
		case <-ctx.Done():
			return
		case frame, ok := <-frameCh:
			if !ok {
				return
			}
			_, err := fmt.Fprintf(w, "--frame\r\nContent-Type: image/jpeg\r\nContent-Length: %d\r\n\r\n", len(frame))
			if err != nil {
				return
			}
			if _, err = w.Write(frame); err != nil {
				return
			}
			if _, err = w.Write([]byte("\r\n")); err != nil {
				return
			}
			if flusher, ok := w.(http.Flusher); ok {
				flusher.Flush()
			}
		}
	}
}

func (h *Handlers) respondError(w http.ResponseWriter, statusCode int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status": statusCode,
		"error":  http.StatusText(statusCode),
	})
}
