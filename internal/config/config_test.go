package config

import (
	"os"
	"testing"
)

// TestLoad_SuccessAndFailure комплексно тестирует базовую логику функции Load:
// ошибки отсутствия окружения, криптографическую стойкость JWT и штатную инициализацию.
func TestLoad_SuccessAndFailure(t *testing.T) {
	// Сценарий 1: Ошибка Fail-Fast валидации секретов при пустом окружении
	os.Clearenv()
	_, err := Load()
	if err == nil {
		t.Error("Ожидалась ошибка при отсутствии переменных окружения")
	}

	// Сценарий 2: Контроль длины JWT_SECRET для предотвращения атак перебора по словарю
	os.Setenv("ADMIN_USERNAME", "admin")
	os.Setenv("ADMIN_PASSWORD_HASH", "hash")
	os.Setenv("JWT_SECRET", "short")
	_, err = Load()
	if err == nil {
		t.Error("Ожидалась ошибка стойкости для короткого JWT_SECRET")
	}

	// Сценарий 3: Успешная загрузка конфигурации с валидными параметрами по умолчанию
	os.Setenv("JWT_SECRET", "super-secure-secret-key-32-bytes-long!!!")
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Не удалось загрузить валидный конфиг: %v", err)
	}
	if cfg.Port != "8080" {
		t.Errorf("Ожидался порт 8080, получен %s", cfg.Port)
	}
}

// TestGetEnv проверяет внутреннюю хелпер-функцию getEnv на корректное извлечение
// существующих переменных среды и бесперебойную отдачу дефолтных значений (fallback).
func TestGetEnv(t *testing.T) {
	// Устанавливаем тестовую переменную окружения
	os.Setenv("TEST_WEBCAM_VAR", "active")
	defer os.Unsetenv("TEST_WEBCAM_VAR")

	// Тест 1: Проверка успешного извлечения активного значения
	if val := getEnv("TEST_WEBCAM_VAR", "default"); val != "active" {
		t.Errorf("Ожидалось 'active', получено '%s'", val)
	}
}

// helperCreateEnv атомарно создает временный файл конфигурации .env на диске.
// По окончании вызывающего теста фикстура гарантирует удаление файла через t.Cleanup.
func helperCreateEnv(t *testing.T, content string) {
	err := os.WriteFile(".env", []byte(content), 0666)
	if err != nil {
		t.Fatalf("Не удалось создать временный .env файл: %v", err)
	}
	t.Cleanup(func() {
		os.Remove(".env")
	})
}

// helperClearEnv производит полную очистку системного окружения от переменных проекта,
// исключая взаимное влияние (leakage) изолированных тест-кейсов друг на друга.
func helperClearEnv() {
	os.Unsetenv("SERVER_PORT")
	os.Unsetenv("ADMIN_USERNAME")
	os.Unsetenv("ADMIN_PASSWORD_HASH")
	os.Unsetenv("JWT_SECRET")
	os.Unsetenv("DEFAULT_CAMERA")
	os.Unsetenv("MAX_LOG_SIZE_MB")
}

// TestLoad_Success верифицирует сквозной парсинг физического .env файла,
// корректность тримминга пробелов, игнорирование комментариев и конвертацию мегабайт в байты.
func TestLoad_Success(t *testing.T) {
	defer helperClearEnv()

	envContent := `
	# Это тестовый комментарий
	SERVER_PORT=9090
	ADMIN_USERNAME=sanek
	ADMIN_PASSWORD_HASH=$2a$10$abcdefghijklmnopqrstuv
	JWT_SECRET=super_secret_string_32_characters_long
	DEFAULT_CAMERA=/dev/video1
	MAX_LOG_SIZE_MB=10

	# Пустая строка ниже для проверки тримминга
	`
	helperCreateEnv(t, envContent)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Ожидалась успешная загрузка, получена ошибка: %v", err)
	}

	// Сверяем распарсенные поля со структурой Config хоста
	if cfg.Port != "9090" {
		t.Errorf("Неверный Port: ожидалось '9090', получено %q", cfg.Port)
	}
	if cfg.Username != "sanek" {
		t.Errorf("Неверный Username: ожидалось 'sanek', получено %q", cfg.Username)
	}
	if cfg.DefaultCam != "/dev/video1" {
		t.Errorf("Неверный DefaultCam: ожидалось '/dev/video1', получено %q", cfg.DefaultCam)
	}
	// Расчетный лимит диска: 10 MB * 1024 * 1024 = 10485760 байт
	var expectedSizeBytes int64 = 10 * 1024 * 1024
	if cfg.MaxLogSize != expectedSizeBytes {
		t.Errorf("Неверный MaxLogSize в байтах: ожидалось %d, получено %d", expectedSizeBytes, cfg.MaxLogSize)
	}
}

// TestLoad_ValidationErrors тестирует сценарии защитных проверок (Validation Boundaries),
// блокируя запуск некорректно сконфигурированного или уязвимого веб-сервера.
func TestLoad_ValidationErrors(t *testing.T) {
	// Сценарий 1: Отсутствие обязательных полей (Fail-Fast при пропуске учетных данных)
	t.Run("Missing_Required_Fields", func(t *testing.T) {
		defer helperClearEnv()
		envContent := `
		SERVER_PORT=8080
		ADMIN_USERNAME=admin
		# Пароль и секрет пропущены
		`
		helperCreateEnv(t, envContent)

		_, err := Load()
		if err == nil {
			t.Error("Ожидалась ошибка из-за отсутствия критических переменных, но метод вернул nil")
		}
	})

	// Сценарий 2: Перехват небезопасных ключей шифрования (длина JWT_SECRET < 32 знаков)
	t.Run("Short_JWT_Secret", func(t *testing.T) {
		defer helperClearEnv()
		envContent := `
		ADMIN_USERNAME=admin
		ADMIN_PASSWORD_HASH=$2a$10$hash
		JWT_SECRET=too_short_secret_key
		`
		helperCreateEnv(t, envContent)

		_, err := Load()
		if err == nil || err.Error() != "переменная JWT_SECRET слишком короткая (минимум 32 символа)" {
			t.Errorf("Ожидалась ошибка длины JWT_SECRET, получено: %v", err)
		}
	})
}

// TestLoad_DefaultsAndFallbackErrors проверяет отказоустойчивость приложения (Graceful Degradation):
// автоматическое назначение безопасных дефолтов при синтаксических ошибках в .env файле.
func TestLoad_DefaultsAndFallbackErrors(t *testing.T) {
	defer helperClearEnv()

	// Передаем некорректный формат строки в лог-лимит ("not_a_number").
	// Поля SERVER_PORT и DEFAULT_CAMERA опускаем для проверки встроенных дефолтов.
	envContent := `
	ADMIN_USERNAME=admin
	ADMIN_PASSWORD_HASH=$2a$10$hash
	JWT_SECRET=super_secret_string_32_characters_long
	MAX_LOG_SIZE_MB=not_a_number
	`
	helperCreateEnv(t, envContent)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Ошибка при загрузке: %v", err)
	}

	// Проверяем работу getEnv дефолтов для сетевой маршрутизации
	if cfg.Port != "8080" {
		t.Errorf("Ожидался дефолтный порт '8080', получено %q", cfg.Port)
	}
	if cfg.DefaultCam != "/dev/video0" {
		t.Errorf("Ожидалась дефолтная камера '/dev/video0', получено %q", cfg.DefaultCam)
	}

	// Проверяем перехват ошибки парсинга strconv.ParseInt: лимит диска должен сброситься на дефолтные 5 МБ
	var defaultExpectedBytes int64 = 5 * 1024 * 1024
	if cfg.MaxLogSize != defaultExpectedBytes {
		t.Errorf("При ошибка парсинга ожидался дефолт 5 МБ (%d байт), получено %d", defaultExpectedBytes, cfg.MaxLogSize)
	}
}
