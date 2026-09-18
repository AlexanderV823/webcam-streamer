package auth

import (
	"strings"
	"testing"
	
	"webcam-streamer/internal/config"
	"golang.org/x/crypto/bcrypt"
)

func TestAuthUsecase(t *testing.T) {
	password := "super_secret_pass"
	// Генерируем тестовый хэш
	hashBytes, _ := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)

	cfg := &config.Config{
		Username:     "admin",
		PasswordHash: string(hashBytes),
		JWTSecret:    "my_test_secret_key_32_bytes_long_!!!",
	}

	uc := NewAuthUsecase(cfg)

	// 1. Тест успешного логина
	token, err := uc.Login("admin", password)
	if err != nil {
		t.Fatalf("Ожидался успешный вход, получен фидбек: %v", err)
	}

	if !strings.Contains(token, ".") || len(strings.Split(token, ".")) != 3 {
		t.Fatalf("Сгенерирован некорректный формат JWT токена")
	}

	// 2. Тест валидации корректного токена
	if !uc.ValidateToken(token) {
		t.Errorf("Валидный токен не прошёл верификацию подписи")
	}

	// 3. Тест неверного пароля
	_, err = uc.Login("admin", "wrong_password")
	if err == nil {
		t.Errorf("Ожидалась ошибка авторизации для неверного пароля")
	}

	// 4. Тест поддельного токена
	fakeToken := token + "malicious_edit"
	if uc.ValidateToken(fakeToken) {
		t.Errorf("Система верифицировала поддельный/измененный токен!")
	}
}
