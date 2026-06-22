package media

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// TargetStorage описывает контракт для физического сохранения бэкап-сетов
type TargetStorage interface {
	// OpenSession создает архивный файл или объект для конкретной сессии бэкапа
	OpenSession(sessionID string) (io.WriteCloser, error)
}

// LocalDiskStorage реализует хранение бэкапов в локальной директории сервера
type LocalDiskStorage struct {
	BaseDir string
}

func NewLocalDiskStorage(baseDir string) (*LocalDiskStorage, error) {
	if err := os.MkdirAll(baseDir, 0755); err != nil {
		return nil, err
	}
	return &LocalDiskStorage{BaseDir: baseDir}, nil
}

func (lds *LocalDiskStorage) OpenSession(sessionID string) (io.WriteCloser, error) {
	archivePath := filepath.Join(lds.BaseDir, fmt.Sprintf("%s.dpbak.zst", sessionID))
	return os.Create(archivePath)
}
