package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"golang.org/x/crypto/bcrypt"
	"webcam-streamer/internal/config"
	"webcam-streamer/internal/domain"
	"webcam-streamer/internal/usecase/auth"
	"webcam-streamer/internal/usecase/stream"
)

// testCapture реализует domain.VideoCapture для использования в HTTP-тестах
type testCapture struct{}

// testCaptureErr имитирует аппаратный сбой инициализации камеры
type testCaptureErr struct {
	testCapture
	initErr bool
}

// testScannerErr имитирует сбой сканирования шины USB
type testScannerErr struct {
	testScanner
	returnErr bool
}

func (tc *testCapture) Init(_ string) error        { return nil }
func (tc *testCapture) ReadFrame() ([]byte, error) { return []byte{0xFF, 0xD8, 0xFF}, nil }
func (tc *testCapture) Close() error               { return nil }
func (e *testCaptureErr) Init(_ string) error      { return errors.New("v4l2 ioctl hardware crash") }
func (e *testScannerErr) Scan() ([]domain.DeviceInfo, error) {
	return nil, errors.New("sysfs read permissions denied")
}

// testScanner реализует domain.CameraScanner для использования в HTTP-тестах
type testScanner struct{}

func (ts *testScanner) Scan() ([]domain.DeviceInfo, error) {
	return []domain.DeviceInfo{
		{ID: "/dev/video0", Name: "Тестовая камера Логитек"},
	}, nil
}

// инициируем обертку для поддержки интерфейса http.Flusher внутри тестов
type flushRecorder struct {
	*httptest.ResponseRecorder
	flushed chan bool
}

func (fr *flushRecorder) Flush() {
	fr.ResponseRecorder.Flush()
	// Сигнализируем тесту, что хэндлер успешно обработал и вытолкнул кадр в сеть
	select {
	case fr.flushed <- true:
	default:
	}
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

// TestHandlers_ValidationErrors тестирует ошибоки синтаксиса JSON и некорректных методов для 100% покрытия
func TestHandlers_ValidationErrors(t *testing.T) {
	h := NewHandlers(nil, nil)

	// 1. Ошибка декодирования тела JSON в HandleLogin (BadRequest 400)
	t.Run("Login_Invalid_JSON", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/api/login", bytes.NewBufferString("{invalid-json"))
		w := httptest.NewRecorder()
		h.HandleLogin(w, req)
		if w.Code != http.StatusBadRequest {
			t.Errorf("Ожидался статус 400, получен %d", w.Code)
		}
	})

	// 2. Невалидный метод для получения списка камер (405 Method Not Allowed)
	t.Run("ListCameras_Invalid_Method", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/api/cameras", nil)
		w := httptest.NewRecorder()
		h.HandleListCameras(w, req)
		if w.Code != http.StatusMethodNotAllowed {
			t.Errorf("Ожидался статус 405, получен %d", w.Code)
		}
	})

	// 3. Невалидный метод для переключения камеры (405 Method Not Allowed)
	t.Run("SwitchCamera_Invalid_Method", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/cameras/switch", nil)
		w := httptest.NewRecorder()
		h.HandleSwitchCamera(w, req)
		if w.Code != http.StatusMethodNotAllowed {
			t.Errorf("Ожидался статус 405, получен %d", w.Code)
		}
	})

	// 4. Ошибка декодирования JSON в HandleSwitchCamera (BadRequest 400)
	t.Run("SwitchCamera_Invalid_JSON", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/api/cameras/switch", bytes.NewBufferString("{invalid-json"))
		w := httptest.NewRecorder()
		h.HandleSwitchCamera(w, req)
		if w.Code != http.StatusBadRequest {
			t.Errorf("Ожидался статус 400, получен %d", w.Code)
		}
	})

	// 5. Симуляция внутренней ошибки UseCase при запросе списка камер (HTTP 500)
	t.Run("ListCameras_Internal_Error", func(t *testing.T) {
		// Создаем сканер, который принудительно вернет ошибку, а не упадет в панику
		badScanner := &testScannerErr{returnErr: true}
		badStreamUC := stream.NewStreamUsecase(&testCapture{}, badScanner, "/dev/video0")
		badH := NewHandlers(nil, badStreamUC)

		req := httptest.NewRequest(http.MethodGet, "/api/cameras", nil)
		w := httptest.NewRecorder()
		badH.HandleListCameras(w, req)

		if w.Code != http.StatusInternalServerError {
			t.Errorf("Ожидался статус 500 Internal Server Error, получен %d", w.Code)
		}
	})

	// 6. Симуляция внутренней ошибки железа при смене камеры (HTTP 500)
	t.Run("SwitchCamera_Internal_Error", func(t *testing.T) {
		// Создаем камеру, которая принудительно выдаст ошибку инициализации при переключении
		badCapture := &testCaptureErr{initErr: true}
		badStreamUC := stream.NewStreamUsecase(badCapture, &testScanner{}, "/dev/video0")
		badH := NewHandlers(nil, badStreamUC)

		switchData := switchCamReq{CameraID: "/dev/video1"} // Меняем путь, чтобы сработало условие переключения
		switchBytes, _ := json.Marshal(switchData)

		req := httptest.NewRequest(http.MethodPost, "/api/cameras/switch", bytes.NewBuffer(switchBytes))
		w := httptest.NewRecorder()
		badH.HandleSwitchCamera(w, req)

		if w.Code != http.StatusInternalServerError {
			t.Errorf("Ожидался статус 500 Internal Server Error, получен %d", w.Code)
		}
	})
}

// TestHandleStream_Cancellation проверяет MJPEG заголовки и безопасный выход из бесконечного цикла
func TestHandleStream_Cancellation(t *testing.T) {
	// Инициализируем UseCase с вашей тестовой камерой, выдающей кадры
	streamUC := stream.NewStreamUsecase(&testCapture{}, &testScanner{}, "/dev/video0")
	h := NewHandlers(nil, streamUC)

	// Создаем контекст, который мы отменим, как только получим данные
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// 1. ЗАПУСКАЕМ БРОДКАСТЕР. Он начнет циклично читать кадры из testCapture
	go streamUC.StartBroadcast(ctx)

	// Создаем HTTP-запрос, привязанный к нашему контексту отмены
	req := httptest.NewRequest(http.MethodGet, "/api/stream", nil)
	req = req.WithContext(ctx)
	w := httptest.NewRecorder()

	// 2. Асинхронно следим за наполнением буфера ответа.
	// Как только хэндлер запишет туда кадр с границей --frame, мы прервем цикл через cancel()
	go func() {
		ticker := time.NewTicker(2 * time.Millisecond)
		defer ticker.Stop()

		// Задаем жесткий таймаут безопасности 200мс, чтобы тест не завис в случае сбоя
		timeout := time.After(200 * time.Millisecond)

		for {
			select {
			case <-ticker.C:
				if bytes.Contains(w.Body.Bytes(), []byte("--frame")) {
					cancel() // Успех: данные получены, останавливаем хэндлер
					return
				}
			case <-timeout:
				cancel() // Аварийный выход по таймауту
				return
			}
		}
	}()

	// Вызываем тестируемый обработчик. Он будет крутиться, пока горутина выше не вызовет cancel()
	h.HandleStream(w, req)

	// Проверяем, что хэндлер успел выставить правильный Content-Type для трансляции изображений
	if w.Header().Get("Content-Type") != "multipart/x-mixed-replace; boundary=frame" {
		t.Error("HandleStream не выставил заголовок multipart/x-mixed-replace для MJPEG")
	}

	// Верифицируем, что в теле ответа присутствуют валидные multipart границы кадра
	if !bytes.Contains(w.Body.Bytes(), []byte("--frame")) {
		t.Errorf("Поток вывода HandleStream не содержит границ кадров MJPEG (--frame). Получено байт: %d", w.Body.Len())
	}
}
