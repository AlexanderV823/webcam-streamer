package middleware

import (
	"os"
	"sync"
)

// RotatingFileWriter управляет записью в файл с контролем размера
type RotatingFileWriter struct {
	filename string
	maxSize  int64
	mu       sync.Mutex
}

// NewRotatingFileWriter инициализирует новый экземпляр RotatingFileWriter с ограничением размера.
func NewRotatingFileWriter(filename string, maxSize int64) *RotatingFileWriter {
	return &RotatingFileWriter{
		filename: filename,
		maxSize:  maxSize,
	}
}

// Write проверяет размер файла перед записью. Если размер превышен — файл перезаписывается.
func (r *RotatingFileWriter) Write(p []byte) (n int, err error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Проверяем текущий размер файла, если он существует
	if info, err := os.Stat(r.filename); err == nil {
		if info.Size()+int64(len(p)) > r.maxSize {
			// Превышен лимит: открываем файл с флагом O_TRUNC для полной очистки (перезаписи)
			file, err := os.OpenFile(r.filename, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0666)
			if err != nil {
				return 0, err
			}
			defer file.Close()
			return file.Write(p)
		}
	}

	// Обычный режим: открываем файл в режиме добавления строк (Append)
	file, err := os.OpenFile(r.filename, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		return 0, err
	}
	defer file.Close()

	return file.Write(p)
}
