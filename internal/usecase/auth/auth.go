// Package auth реализует логику аутентификации пользователей и генерации/валидации JWT-токенов.
package auth

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"
	"webcam-streamer/internal/config"
)

// ErrAuthFailed возвращается при неверном логине или пароле.
var ErrAuthFailed = errors.New("authentication failed")

// Usecase координирует процессы проверки прав и выпуска токенов доступа.
type Usecase struct {
	cfg *config.Config
}

// NewAuthUsecase создает новый экземпляр UseCase для работы с авторизацией.
func NewAuthUsecase(cfg *config.Config) *Usecase {
	return &Usecase{cfg: cfg}
}

// JWTClaims описывает структуру полезной нагрузки (payload) выпускаемых JWT-токенов.
type JWTClaims struct {
	Sub string `json:"sub"`
	Exp int64  `json:"exp"`
}

// Login сверяет введенный пароль с сохраненным Bcrypt-хэшем и возвращает JWT-токен.
func (u *Usecase) Login(username, password string) (string, error) {
	if username != u.cfg.Username {
		return "", ErrAuthFailed
	}

	err := bcrypt.CompareHashAndPassword([]byte(u.cfg.PasswordHash), []byte(password))
	if err != nil {
		return "", ErrAuthFailed
	}

	headerJSON, _ := json.Marshal(map[string]string{"alg": "HS256", "typ": "JWT"})
	header := base64.RawURLEncoding.EncodeToString(headerJSON)

	claims := JWTClaims{
		Sub: username,
		Exp: time.Now().Add(24 * time.Hour).Unix(),
	}
	payloadJSON, _ := json.Marshal(claims)
	payload := base64.RawURLEncoding.EncodeToString(payloadJSON)

	unsignedToken := header + "." + payload
	mac := hmac.New(sha256.New, []byte(u.cfg.JWTSecret))
	mac.Write([]byte(unsignedToken))
	signature := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))

	return unsignedToken + "." + signature, nil
}

// ValidateToken выполняет полную проверку криптографической подписи JWT и срока его действия.
func (u *Usecase) ValidateToken(token string) bool {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return false
	}

	// Корректная сборка заголовка и полезной нагрузки для проверки подписи
	unsignedToken := parts[0] + "." + parts[1]
	mac := hmac.New(sha256.New, []byte(u.cfg.JWTSecret))
	mac.Write([]byte(unsignedToken))
	expectedSignature := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))

	if !hmac.Equal([]byte(parts[2]), []byte(expectedSignature)) {
		return false
	}

	payloadBytes, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return false
	}

	var claims JWTClaims
	if err := json.Unmarshal(payloadBytes, &claims); err != nil {
		return false
	}

	return time.Now().Unix() < claims.Exp
}
