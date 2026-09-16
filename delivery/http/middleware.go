package http

import (
	"log"
	"net/http"
	"time"
)

// responseWriterInterceptor оборачивает http.ResponseWriter для перехвата HTTP-статуса
type responseWriterInterceptor struct {
	http.ResponseWriter
	statusCode int
}

// NewResponseWriterInterceptor создает новую обертку с дефолтным статусом 200 OK
func NewResponseWriterInterceptor(w http.ResponseWriter) *responseWriterInterceptor {
	return &responseWriterInterceptor{w, http.StatusOK}
}

// WriteHeader перехватывает код ответа перед отправкой его клиенту
func (rw *responseWriterInterceptor) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

// LoggingMiddleware оборачивает http.Handler для логирования запросов, IP и HTTP-статусов
func LoggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		// Извлекаем реальный IP-адрес клиента, переданный через Nginx
		ip := r.Header.Get("X-Real-IP")
		if ip == "" {
			ip = r.Header.Get("X-Forwarded-For")
		}
		if ip == "" {
			ip = r.RemoteAddr
		}

		// Оборачиваем стандартный ResponseWriter в наш перехватчик
		interceptor := NewResponseWriterInterceptor(w)

		// Передаем управление дальше по цепочке
		next.ServeHTTP(interceptor, r)

		// Логируем результат с IP, методом, путем, статусом и временем выполнения
		log.Printf(
			"[%s] %s %s -> %d %s | Длительность: %v",
			ip,
			r.Method,
			r.URL.Path,
			interceptor.statusCode,
			http.StatusText(interceptor.statusCode),
			time.Since(start),
		)
	})
}
