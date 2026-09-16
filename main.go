package main

import (
	"fmt"
	"log"
	"os"
	"net/http"
	delivery "webcam-streamer/delivery/http"
	"webcam-streamer/infrastructure/v4l2"
	"webcam-streamer/usecase"
)

func main() {
	cameraRepo := v4l2.NewV4L2Repository()
	cameraStreamer := v4l2.NewV4L2Streamer()
	cameraUC := usecase.NewCameraUseCase(cameraRepo, cameraStreamer)
	httpHandler := delivery.NewHTTPHandler(cameraUC)

	mux := http.NewServeMux()
	httpHandler.RegisterRoutes(mux)

	// Читаем порт из окружения Docker
	port := os.Getenv("APP_PORT")
	if port == "" {
		port = "8080"
	}

	fmt.Printf("Легковесный стример запущен внутри контейнера на http://localhost:%s\n", port)
	if err := http.ListenAndServe(":"+port, mux); err != nil {
		log.Fatalf("Ошибка сервера: %v", err)
	}
}
