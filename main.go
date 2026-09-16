// Главный пакет приложения, отвечающий за сборку и старт веб-сервера.
package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
	delivery "webcam-streamer/delivery/http"
	"webcam-streamer/infrastructure/ffmpeg"
	"webcam-streamer/usecase"
)

// main выполняет сборку графа зависимостей (DI) и запускает прослушивание HTTP-порта.
func main() {
	cameraRepo := ffmpeg.NewFFmpegRepository()
	cameraStreamer := ffmpeg.NewFFmpegStreamer()

	cameraUC := usecase.NewCameraUseCase(cameraRepo, cameraStreamer)
	httpHandler := delivery.NewHTTPHandler(cameraUC)

	mux := http.NewServeMux()
	handlerWithLogging := httpHandler.RegisterRoutes(mux)

	port := os.Getenv("APP_PORT")
	if port == "" {
		port = "8080"
	}

	// Конфигурируем HTTP-сервер для возможности управления его жизненным циклом
	srv := &http.Server{
		Addr:    ":" + port,
		Handler: handlerWithLogging,
	}

	// Канал для перехвата системных сигналов завершения от ОС или Docker
	shutdownSignals := make(chan os.Signal, 1)
	// Подписываемся на SIGINT (Ctrl+C) и SIGTERM (команда остановки контейнера)
	signal.Notify(shutdownSignals, os.Interrupt, syscall.SIGTERM)

	// Запускаем веб-сервер в отдельной горутине, чтобы он не блокировал основной поток
	go func() {
		fmt.Printf("Полностью кроссплатформенный FFmpeg-стример запущен на http://localhost:%s\n", port)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("Ошибка сервера: %v", err)
		}
	}()

	// Блокируем основной поток и ждем сигнал на завершение работы
	sig := <-shutdownSignals
	fmt.Printf("\nПолучен сигнал %v. Начинаем плавное завершение работы веб-сервера...\n", sig)

	// Выделяем серверу 10 секунд на то, чтобы закрыть текущие MJPEG-трансляции клиентов
	// Прерывание сетевых запросов автоматически завершит процессы ffmpeg в ОС
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Начинаем процедуру Shutdown
	if err := srv.Shutdown(ctx); err != nil {
		log.Printf("Ошибка при плавном завершении сервера: %v", err)
		_ = srv.Close() // Жесткое закрытие в случае зависания
	}

	fmt.Println("Веб-сервер успешно и безопасно остановлен.")
}
