package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"webcam-streamer/internal/config"
	"webcam-streamer/internal/delivery/embed"
	"webcam-streamer/internal/delivery/http/handlers"
	"webcam-streamer/internal/delivery/http/middleware"
	"webcam-streamer/internal/usecase/auth"
	"webcam-streamer/internal/usecase/stream"
	"webcam-streamer/pkg/camera"
)

func main() {
	log.Println("[INIT] Инициализация системы видеотрансляции...")

	// 1. Загрузка и строгая валидация конфигурации из .env
	cfg := config.Load()

	// 2. Автосканирование доступных физических USB-устройств в системе
	log.Println("[INIT] Сканирование доступных USB-веб-камер...")
	devices, err := camera.ScanDevices()
	if err != nil {
		log.Printf("[WARN] Не удалось выполнить сканирование устройств: %v", err)
	} else {
		log.Printf("[INIT] Найдено камер в системе: %d", len(devices))
		for _, dev := range devices {
			log.Printf("  -> [%s] %s", dev.ID, dev.Name)
		}
	}

	// 3. Инициализация слоя инфраструктуры (драйвер камеры)
	camDevice := camera.NewCamera()

	// Пробуем запустить камеру по умолчанию из конфигурации (.env)
	log.Printf("[INIT] Попытка активации камеры по умолчанию: %s", cfg.DefaultCam)
	if err := camDevice.Init(cfg.DefaultCam); err != nil {
		log.Printf("[WARN] Камера по умолчанию (%s) недоступна: %v. Ожидание выбора пользователя через UI.", cfg.DefaultCam, err)
	}
	defer camDevice.Close()

	// 4. Инициализация бизнес-логики (Use Cases)
	authUC := auth.NewAuthUsecase(cfg)
	streamUC := stream.NewStreamUsecase(camDevice, cfg.DefaultCam)

	// 5. Запуск независимого конкурентного процесса захвата и вещания кадров
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go streamUC.StartBroadcast(ctx)

	// 6. Инициализация слоев адаптеров доставки (HTTP и Middleware)
	h := handlers.NewHandlers(authUC, streamUC)
	mw := middleware.NewMiddleware(authUC)

	// 7. Конфигурация маршрутизации (Стандартный Multiplexer Go)
	mux := http.NewServeMux()

	// Публичные эндпоинты с обязательной защитой от брутфорса (Rate Limiting)
	mux.Handle("/api/login", mw.RateLimiter(http.HandlerFunc(h.HandleLogin)))

	// Защищенные приватные эндпоинты, требующие валидный JWT-токен
	mux.Handle("/api/stream", mw.AuthRequired(http.HandlerFunc(h.HandleStream)))
	mux.Handle("/api/cameras", mw.AuthRequired(http.HandlerFunc(h.HandleListCameras)))
	mux.Handle("/api/cameras/switch", mw.AuthRequired(http.HandlerFunc(h.HandleSwitchCamera)))

	// Раздача статического фронтенда, скомпилированного прямо в бинарник (go embed)
	mux.Handle("/", http.FileServer(http.FS(embed.WebUI)))

	// Обертываем все эндпоинты в глобальный логгер входящих IP-запросов
	siteHandler := mw.Logger(mux)

	// 8. Конфигурация HTTP-сервера
	srv := &http.Server{
		Addr:         ":" + cfg.Port,
		Handler:      siteHandler,
		WriteTimeout: 0, // Устанавливаем 0, так как MJPEG-трансляция является бесконечным потоком данных
		ReadTimeout:  15 * time.Second,
	}

	// Запуск сервера в отдельной горутине, чтобы не блокировать основной поток
	go func() {
		log.Printf("[SUCCESS] HTTP-сервер успешно запущен на порту %s", cfg.Port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("[FATAL] Ошибка запуска HTTP-сервера: %v", err)
		}
	}()

	// 9. Реализация Graceful Shutdown (Безопасная остановка процесса без потери данных)
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)

	// Ожидаем системный сигнал прерывания (Ctrl+C или kill)
	<-stop
	log.Println("[SHUTDOWN] Получен сигнал остановки. Корректное завершение процессов...")

	// Даем серверу максимум 5 секунд на закрытие текущих активных сетевых сессий
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Printf("[ERROR] Ошибка при принудительной остановке сервера: %v", err)
	}

	log.Println("[SUCCESS] Сервер трансляции успешно остановлен и очищен.")
}
