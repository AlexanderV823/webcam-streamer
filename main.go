// Главный пакет приложения, отвечающий за сборку и старт веб-сервера.
package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	delivery "webcam-streamer/delivery/http"
	"webcam-streamer/infrastructure/v4l2"
	"webcam-streamer/usecase"
)

// main выполняет сборку графа зависимостей (DI) и запускает прослушивание HTTP-порта.
func main() {
	cameraRepo := v4l2.NewV4L2Repository()
	cameraStreamer := v4l2.NewV4L2Streamer()

	cameraUC := usecase.NewCameraUseCase(cameraRepo, cameraStreamer)

	httpHandler := delivery.NewHTTPHandler(cameraUC)

	mux := http.NewServeMux()
	handlerWithLogging := httpHandler.RegisterRoutes(mux)

	// Читаем порт из окружения Docker
	port := os.Getenv("APP_PORT")
	if port == "" {
		port = "8080"
	}

	fmt.Printf("Легковесный стример запущен внутри контейнера на http://localhost:%s\n", port)
	// Передаем handlerWithLogging вместо mux
	if err := http.ListenAndServe(":"+port, handlerWithLogging); err != nil {
		log.Fatalf("Ошибка сервера: %v", err)
	}
}
