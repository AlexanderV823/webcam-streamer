package domain

import (
	"net/http"
	"testing"
)

// TestNewErrorResponse проверяет консистентность формирования структуры ошибок API.
func TestNewErrorResponse(t *testing.T) {
	// Проверяем код 401 Unauthorized
	resp401 := NewErrorResponse(http.StatusUnauthorized)
	if resp401.Status != http.StatusUnauthorized {
		t.Errorf("Ожидался статус 401, получен %d", resp401.Status)
	}
	if resp401.Error != "Unauthorized" {
		t.Errorf("Ожидалась ошибка 'Unauthorized', получена '%s'", resp401.Error)
	}

	// Проверяем код 429 Too Many Requests
	resp429 := NewErrorResponse(http.StatusTooManyRequests)
	if resp429.Status != http.StatusTooManyRequests {
		t.Errorf("Ожидался статус 429, получен %d", resp429.Status)
	}
}
