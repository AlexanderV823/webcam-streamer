package domain

import "net/http"

// ErrorResponse — единая структура ответа в случае ошибки API.
// Передает стандартный HTTP статус-текст и код числом.
type ErrorResponse struct {
	Status int    `json:"status"` // Например, 401, 429, 500
	Error  string `json:"error"`  // Например, "Unauthorized", "Too Many Requests"
}

// NewErrorResponse создает консистентный ответ на основе HTTP-кода
func NewErrorResponse(statusCode int) ErrorResponse {
	return ErrorResponse{
		Status: statusCode,
		Error:  http.StatusText(statusCode),
	}
}
