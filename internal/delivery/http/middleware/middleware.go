package middleware

import (
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"golang.org/x/time/rate"
	"webcam-streamer/internal/usecase/auth"
)

type Middleware struct {
	authUseCase *auth.Usecase
	ips         map[string]*rate.Limiter
	mu          sync.Mutex
}

func NewMiddleware(au *auth.Usecase) *Middleware {
	return &Middleware{
		authUseCase: au,
		ips:         make(map[string]*rate.Limiter),
	}
}

// Logger перехватывает запросы и логирует метод, путь, входящий IP и время обработки
func (m *Middleware) Logger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		// Извлекаем чистый IP (учитывая возможный прокси-сервер типа Nginx)
		ip := r.Header.Get("X-Forwarded-For")
		if ip == "" {
			ip = r.RemoteAddr
			// Отсекаем порт, оставляя только IP-адрес
			if idx := strings.LastIndex(ip, ":"); idx != -1 {
				ip = ip[:idx]
			}
		}

		next.ServeHTTP(w, r)

		log.Printf("[HTTP LOG] %s -- %s %s -- от IP: %s -- Заняло: %v",
			time.Now().Format("2006-01-02 15:04:05"),
			r.Method,
			r.URL.Path,
			ip,
			time.Since(start),
		)
	})
}

// RateLimiter защищает эндпоинты (например, /api/login) от брутфорса
func (m *Middleware) RateLimiter(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Выделяем IP для лимитирования
		ip := r.Header.Get("X-Forwarded-For")
		if ip == "" {
			ip = r.RemoteAddr
			if idx := strings.LastIndex(ip, ":"); idx != -1 {
				ip = ip[:idx]
			}
		}

		m.mu.Lock()
		limiter, exists := m.ips[ip]
		if !exists {
			// Разрешаем максимум 3 запроса в секунду с возможностью всплеска (burst) до 5 запросов
			limiter = rate.NewLimiter(rate.Every(time.Second/3), 5)
			m.ips[ip] = limiter
		}
		m.mu.Unlock()

		if !limiter.Allow() {
			// Логируем попытку брутфорса/атаки
			log.Printf("[SECURITY WARNING] Превышен лимит запросов (Брутфорс атака?) от IP: %s на %s", ip, r.URL.Path)

			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusTooManyRequests)
			w.Write([]byte(`{"status":429,"error":"Too Many Requests"}`))
			return
		}
		next.ServeHTTP(w, r)
	})
}

// AuthRequired проверяет наличие и валидность JWT-токена
func (m *Middleware) AuthRequired(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Токен может передаваться как в Query-параметре (для тега <img>), так и в заголовках
		token := r.URL.Query().Get("token")
		if token == "" {
			if authHeader := r.Header.Get("Authorization"); strings.HasPrefix(authHeader, "Bearer ") {
				token = strings.TrimPrefix(authHeader, "Bearer ")
			}
		}

		if token == "" || !m.authUseCase.ValidateToken(token) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			w.Write([]byte(`{"status":401,"error":"Unauthorized"}`))
			return
		}
		next.ServeHTTP(w, r)
	})
}
