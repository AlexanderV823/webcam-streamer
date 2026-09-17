package http

import (
	"crypto/subtle"
	"log"
	"net/http"
	"sync"
	"time"

	"golang.org/x/time/rate"
)

// IPRequestLimiter хранит индивидуальные лимитеры частоты запросов
// для каждого IP-адреса с целью защиты от DoS-атак и брутфорса.
type IPRequestLimiter struct {
	ips struct {
		sync.RWMutex
		v map[string]*rate.Limiter
	}
}

// NewIPRequestLimiter инициализирует и возвращает новый экземпляр IPRequestLimiter
// со сброшенной картой соответствия IP-адресов и их лимитеров.
func NewIPRequestLimiter() *IPRequestLimiter {
	l := &IPRequestLimiter{}
	l.ips.v = make(map[string]*rate.Limiter)
	return l
}

// GetLimiter возвращает существующий лимитер для указанного IP-адреса
// или динамически создает новый, если этот IP обратился впервые.
func (i *IPRequestLimiter) GetLimiter(ip string) *rate.Limiter {
	i.ips.Lock()
	defer i.ips.Unlock()

	limiter, exists := i.ips.v[ip]
	if !exists {
		// rate.Every(time.Second*2) — пополняет баланс на 1 токен каждые 2 секунды.
		// 5 — максимальный «взрывной» объем запросов (burst), который IP может сделать одновременно.
		limiter = rate.NewLimiter(rate.Every(time.Second*2), 5)
		i.ips.v[ip] = limiter
	}
	return limiter
}

// RateLimitMiddleware проверяет IP-адрес клиента и блокирует запрос
// со статусом 429 (Too Many Requests), если превышена допустимая частота обращений.
func (i *IPRequestLimiter) RateLimitMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Получаем реальный IP клиента, учитывая возможный Reverse Proxy (Nginx)
		ip := r.Header.Get("X-Forwarded-For")
		if ip == "" {
			ip = r.RemoteAddr
		}

		// Проверяем, есть ли у данного IP доступный токен для выполнения запроса
		if !i.GetLimiter(ip).Allow() {
			http.Error(w, "Too Many Requests", http.StatusTooManyRequests)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// SecureBasicAuthMiddleware реализует базовую HTTP-авторизацию (Basic Auth),
// устойчивую к атакам по времени (Timing Attacks) благодаря константному времени сравнения строк.
func SecureBasicAuthMiddleware(expectedUser, expectedPass string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		username, password, ok := r.BasicAuth()
		if !ok {
			w.Header().Set("WWW-Authenticate", `Basic realm="Restricted"`)
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		// Сравниваем байты за строго одинаковое время (Constant Time),
		// чтобы злоумышленник не мог угадать пароль по микросекундным задержкам ответа.
		userMatch := subtle.ConstantTimeCompare([]byte(username), []byte(expectedUser)) == 1
		passMatch := subtle.ConstantTimeCompare([]byte(password), []byte(expectedPass)) == 1

		if !userMatch || !passMatch {
			w.Header().Set("WWW-Authenticate", `Basic realm="Restricted"`)
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// LoggingMiddleware перехватывает HTTP-запрос, замеряет время его выполнения
// и записывает в стандартный лог информацию о методе, пути и длительности обработки запроса.
func LoggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		
		// Передаем управление следующему хендлеру по цепочке
		next.ServeHTTP(w, r)
		
		// Логируем результаты выполнения
		log.Printf("[%s] %s %s", r.Method, r.URL.Path, time.Since(start))
	})
}
