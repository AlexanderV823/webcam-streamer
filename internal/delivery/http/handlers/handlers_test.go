package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"webcam-streamer/internal/config"
	"webcam-streamer/internal/domain"
	"webcam-streamer/internal/usecase/auth"
	"webcam-streamer/internal/usecase/stream"
	"golang.org/x/crypto/bcrypt"
)

// testCapture реализует domain.VideoCapture для использования в HTTP-тестах
type testCapture struct{}

func (tc *testCapture) Init(_ string) error      { return nil }
func (tc *testCapture) ReadFrame() ([]byte, error) { return []byte{0xFF, 0xD8, 0xFF}, nil }
func (tc *testCapture) Close() error               { return nil }

// testScanner реализует domain.CameraScanner для использования в HTTP-тестах
type testScanner struct{}

func (ts *testScanner) Scan() ([]domain.DeviceInfo, error) {
	return []domain.DeviceInfo{
		{ID: "/dev/video0", Name: "Тестовая камера Логитек"},
	}, nil
}

func TestHandlers_Workflow(t *testing.T) {
	// Подготавливаем валидный хэш пароля через bcrypt
	password := "secure_admin_password"
	hash, _ := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)

	// Инициализируем конфигурацию для тестов
	cfg := &config.Config{
		Username:     "admin",
		PasswordHash: string(hash),
		JWTSecret:    "test-secret-key-32-bytes-long-for-jwt-signature!!",
	}

	// Собираем слои UseCase с внедрением тестовых интерфейсов
	authUC := auth.NewAuthUsecase(cfg)
	streamUC := stream.NewStreamUsecase(&testCapture{}, &testScanner{}, "/dev/video0")
	h := NewHandlers(authUC, streamUC)

	// --- ТЕСТ 1: POST /api/login (Успешный вход) ---
	loginData := loginReq{Username: "admin", Password: password}
	jsonBytes, _ := json.Marshal(loginData)

	reqLogin := httptest.NewRequest(http.MethodPost, "/api/login", bytes.NewBuffer(jsonBytes))
	reqLogin.Header.Set("Content-Type", "application/json")
	wLogin := httptest.NewRecorder()

	h.HandleLogin(wLogin, reqLogin)

	if wLogin.Code != http.StatusOK {
		t.Errorf("HandleLogin вернул статус %d, ожидался 200", wLogin.Code)
	}

	var loginResp map[string]string
	json.NewDecoder(wLogin.Body).Decode(&loginResp)
	if _, hasToken := loginResp["token"]; !hasToken {
		t.Error("Эндпоинт авторизации не вернул токен в JSON-ответе")
	}

	// --- ТЕСТ 2: GET /api/login (Ошибка: Метод не поддерживается) ---
	reqLoginGet := httptest.NewRequest(http.MethodGet, "/api/login", nil)
	wLoginGet := httptest.NewRecorder()

	h.HandleLogin(wLoginGet, reqLoginGet)

	if wLoginGet.Code != http.StatusMethodNotAllowed {
		t.Errorf("Ожидался статус 405 Method Not Allowed, получен %d", wLoginGet.Code)
	}

	// --- ТЕСТ 3: GET /api/cameras (Получение списка устройств через UseCase) ---
	reqCams := httptest.NewRequest(http.MethodGet, "/api/cameras", nil)
	wCams := httptest.NewRecorder()

	h.HandleListCameras(wCams, reqCams)

	if wCams.Code != http.StatusOK {
		t.Errorf("HandleListCameras вернул статус %d, ожидался 200", wCams.Code)
	}

	var camsResp []domain.DeviceInfo
	json.NewDecoder(wCams.Body).Decode(&camsResp)
	if len(camsResp) != 1 || camsResp[0].ID != "/dev/video0" {
		t.Error("HandleListCameras вернул некорректный список устройств")
	}

	// --- ТЕСТ 4: POST /api/cameras/switch (Переключение камеры на лету) ---
	switchData := switchCamReq{CameraID: "/dev/video0"}
	switchBytes, _ := json.Marshal(switchData)

	reqSwitch := httptest.NewRequest(http.MethodPost, "/api/cameras/switch", bytes.NewBuffer(switchBytes))
	wSwitch := httptest.NewRecorder()

	h.HandleSwitchCamera(wSwitch, reqSwitch)

	if wSwitch.Code != http.StatusOK {
		t.Errorf("HandleSwitchCamera вернул статус %d, ожидался 200", wSwitch.Code)
	}
}

func TestHandleStream_Cancellation(t *testing.T) {
	streamUC := stream.NewStreamUsecase(&testCapture{}, &testScanner{}, "/dev/video0")
	h := NewHandlers(nil, streamUC)

	req := httptest.NewRequest(http.MethodGet, "/api/stream", nil)
	
	// Обертываем в контекст с возможностью отмены, чтобы прервать бесконечный MJPEG-цикл
	ctx, cancel := context.WithCancel(context.Background())
	req = req.WithContext(ctx)
	w := httptest.NewRecorder()

	// Запускаем асинхронную отмену запроса через короткий промежуток времени
	go func() {
		time.Sleep(20 * time.Millisecond)
		cancel()
	}()

	h.HandleStream(w, req)

	// Проверяем, что хэндлер успел выставить правильный Content-Type для трансляции изображений
	if w.Header().Get("Content-Type") != "multipart/x-mixed-replace; boundary=frame" {
		t.Error("HandleStream не выставил заголовок multipart/x-mixed-replace для MJPEG")
	}
}
