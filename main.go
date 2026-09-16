package main

import (
	"fmt"
	"log"
	"net/http"
	delivery "webcam-streamer/delivery/http"
	"webcam-streamer/infrastructure/v4l2"
	"webcam-streamer/usecase"
)

func main() {
	// 1. Инициализируем инфраструктуру (драйверы)
	cameraRepo := v4l2.NewV4L2Repository()
	cameraStreamer := v4l2.NewV4L2Streamer()

	// 2. Инициализируем бизнес-логику (Usecase), внедряя инфраструктуру
	cameraUC := usecase.NewCameraUseCase(cameraRepo, cameraStreamer)

	// 3. Инициализируем транспортный слой (Delivery), внедряя Usecase
	httpHandler := delivery.NewHTTPHandler(cameraUC)

	// 4. Настраиваем маршрутизатор
	mux := http.NewServeMux()
	httpHandler.RegisterRoutes(mux)

	// 5. Запуск HTTPS
	certFile := "server.crt"
	keyFile := "server.key"

	fmt.Println("Легковесный стример (Clear Architecture) запущен на https://localhost:8443")
	if err := http.ListenAndServeTLS(":8443", certFile, keyFile, mux); err != nil {
		log.Fatalf("Ошибка сервера: %v", err)
	}
}
