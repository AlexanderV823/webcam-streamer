// Package http реализует веб-интерфейс приложения, обработку HTTP-маршрутов,
// логирование входящих запросов и формирование MJPEG-потоков.
package http

import (
	"log"
	"net/http"
	"time"
)

// responseWriterInterceptor выступает в роли декоратора для стандартного http.ResponseWriter,
// позволяя перехватывать и сохранять HTTP статус-код ответа для последующего логирования.
type responseWriterInterceptor struct {
	http.ResponseWriter
	statusCode int
}

// NewResponseWriterInterceptor инициализирует перехватчик с базовым успешным статусом 200 OK.
func NewResponseWriterInterceptor(w http.ResponseWriter) *responseWriterInterceptor {
	return &responseWriterInterceptor{w, http.StatusOK}
}

// WriteHeader перехватывает запись HTTP-статуса и сохраняет его во внутреннее поле.
func (rw *responseWriterInterceptor) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

// LoggingMiddleware является промежуточным слоем (Middleware), который вычисляет
// реальный IP-адрес клиента за прокси-сервером Nginx, замеряет время выполнения запроса
// и выводит структурированный лог в стандартный вывод.
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
