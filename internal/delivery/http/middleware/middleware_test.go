package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
	
	"webcam-streamer/internal/config"
	"webcam-streamer/internal/usecase/auth"
)

func TestRateLimiterMiddleware(t *testing.T) {
	cfg := &config.Config{
		Username:     "admin",
		PasswordHash: "$2a$10$something",
		JWTSecret:    "secret_key_32_bytes_long_minimum_!",
	}
	authUC := auth.NewAuthUsecase(cfg)
	mw := NewMiddleware(authUC)

	// Тестовый обработчик, который имитирует успешный проход сквозь лимитер
	dummyHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	limiter := mw.RateLimiter(dummyHandler)

	// Симулируем 10 быстрых запросов подряд с одного IP
	for i := 0; i < 10; i++ {
		req := httptest.NewRequest("POST", "/api/login", nil)
		req.RemoteAddr = "192.168.1.50:1234" // Фиксированный IP атакующего
		rec := httptest.NewRecorder()

		limiter.ServeHTTP(rec, req)

		// По нашему правилу всплеска (burst: 5) первые 5 запросов пройдут (200 OK), 
		// а последующие должны быть заблокированы (429 Too Many Requests)
		if i >= 5 {
			if rec.Code != http.StatusTooManyRequests {
				t.Errorf("Запрос #%d должен быть заблокирован (429), но вернулся статус: %d. Защита от брутфорса не сработала!", i, rec.Code)
			}
		}
	}
}
