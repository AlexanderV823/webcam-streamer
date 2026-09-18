package middleware

import (
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"webcam-streamer/internal/config"
	"webcam-streamer/internal/usecase/auth"
	"golang.org/x/crypto/bcrypt"
)

func TestMiddleware_Logger_And_RateLimiter(t *testing.T) {
	// Очищаем тестовый лог, если он остался
	testLogFile := "log.txt"
	defer os.Remove(testLogFile)

	// 1. Инициализация зависимостей
	password := "admin_pass"
	hash, _ := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	
	cfg := &config.Config{
		Username:     "admin",
		PasswordHash: string(hash),
		JWTSecret:    "secure-test-secret-key-32-bytes-long!!!",
	}

	authUC := auth.NewAuthUsecase(cfg)
	
	// Передаем лимит логов (например, 1024 байта) в обновленный конструктор
	mw := NewMiddleware(authUC, 1024)

	// Создаем тестовый хэндлер, который возвращает статус 200 OK
	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	// 2. Тестируем Logger Middleware
	req := httptest.NewRequest(http.MethodGet, "/api/stream", nil)
	w := httptest.NewRecorder()

	loggerChain := mw.Logger(nextHandler)
	loggerChain.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Ожидался статус 200, получен %d", w.Code)
	}

	// Проверяем, что лог-файл создался и записал событие
	if _, err := os.Stat(testLogFile); os.IsNotExist(err) {
		t.Error("Файл log.txt не был создан системой логирования")
	}

	// 3. Тестируем RateLimiter Middleware
	reqLimit := httptest.NewRequest(http.MethodPost, "/api/login", nil)
	wLimit := httptest.NewRecorder()

	limiterChain := mw.RateLimiter(nextHandler)

	// Имитируем всплеск запросов, чтобы проверить срабатывание защиты от брутфорса
	for i := 0; i < 10; i++ {
		wLimit = httptest.NewRecorder()
		limiterChain.ServeHTTP(wLimit, reqLimit)
	}

	// Последний запрос гарантированно должен натолкнуться на блокировку 429
	if wLimit.Code != http.StatusTooManyRequests {
		t.Errorf("RateLimiter не заблокировал брутфорс-атаку. Получен код %d, ожидался 429", wLimit.Code)
	}
}
