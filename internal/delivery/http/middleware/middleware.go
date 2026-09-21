// Package middleware содержит промежуточные обработчики HTTP-запросов (логирование, защита от атак).
package middleware

import (
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"golang.org/x/time/rate"
	"webcam-streamer/internal/usecase/auth"
)

// Middleware объединяет все защитные и служебные прослойки веб-сервера.
type Middleware struct {
	authUseCase *auth.Usecase
	ips         map[string]*rate.Limiter
	mu          sync.Mutex
	logWriter   io.Writer // Поток для записи логов
}

// NewMiddleware инициализирует Middleware, настраивая сквозное логирование в консоль и файл.
func NewMiddleware(au *auth.Usecase, maxLogSize int64) *Middleware {
	// Создаем наш ротатор для файла log.txt
	fileWriter := NewRotatingFileWriter("log.txt", maxLogSize)

	// Объединяем os.Stdout (консоль) и файл log.txt в один поток
	combinedWriter := io.MultiWriter(os.Stdout, fileWriter)

	// Перенаправляем системный логгер Go на наш объединенный поток
	log.SetOutput(combinedWriter)

	return &Middleware{
		authUseCase: au,
		ips:         make(map[string]*rate.Limiter),
		logWriter:   combinedWriter,
	}
}

func getRealIP(r *http.Request) string {
	// 1. Первым делом смотрим на заголовок, который жестко прописан в Nginx.
	// Nginx берет его из сетевого соединения ($remote_addr), хакер не может его подделать.
	ip := r.Header.Get("X-Real-IP")
	if ip != "" {
		return strings.TrimSpace(ip)
	}

	// 2. Если X-Real-IP пустой (например, запустили Go-приложение локально без Nginx),
	// откатываемся на базовый RemoteAddr дескриптора.
	ip = r.RemoteAddr
	if idx := strings.LastIndex(ip, ":"); idx != -1 {
		ip = ip[:idx]
	}
	return ip
}

// Logger перехватывает HTTP-запросы для ведения логов времени обработки, путей и IP-адресов.
func (m *Middleware) Logger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		ip := getRealIP(r)

		next.ServeHTTP(w, r)

		// Запись автоматически запишется и в stdout, и в log.txt
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
		ip := getRealIP(r)

		m.mu.Lock()
		limiter, exists := m.ips[ip]
		if !exists {
			limiter = rate.NewLimiter(rate.Every(time.Second/3), 5)
			m.ips[ip] = limiter
		}
		m.mu.Unlock()

		if !limiter.Allow() {
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
