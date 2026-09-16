// Главный пакет приложения, отвечающий за сборку и старт веб-сервера.
package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
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

	fmt.Printf("Полностью кроссплатформенный FFmpeg-стример запущен на http://localhost:%s\n", port)
	if err := http.ListenAndServe(":"+port, handlerWithLogging); err != nil {
		log.Fatalf("Ошибка сервера: %v", err)
	}
}
