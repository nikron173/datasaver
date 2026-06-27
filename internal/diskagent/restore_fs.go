package diskagent

import (
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/nikron173/datasaver/internal/archive"
)

// RestoreSource описывает контракт для получения данных восстановления (из файла или сети)
type RestoreSource interface {
	// NextFile переводит источник к следующему файлу в потоке.
	// Возвращает заголовки файла (размер, права) и его оригинальный путь.
	// Если файлы закончились, метод должен вернуть ошибку io.EOF.
	NextFile() (meta *archive.FileHeader, path string, err error)

	// ReadChunk читает очередной кусок (чанк) содержимого ТЕКУЩЕГО файла.
	// Он должен вести себя как стандартный io.Reader: возвращать количество прочитанных байт
	// и io.EOF, когда тело конкретно этого файла полностью вычитано из чанков.
	ReadChunk(buf []byte) (n int, err error)

	// Close корректно закрывает источник (локальный файл или gRPC соединение)
	Close() error
}

type FileSystemRestore struct {
	Source     RestoreSource
	RestoreDir string
}

func NewFileSystemRestore(restoreDir string, source RestoreSource) *FileSystemRestore {
	return &FileSystemRestore{RestoreDir: restoreDir, Source: source}
}

func (fsr *FileSystemRestore) Run() error {
	for {
		meta, originalPath, err := fsr.Source.NextFile()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			slog.Error("error read file metadata", slog.String("err", err.Error()))
			return err
		}

		finalPath := originalPath
		if fsr.RestoreDir != "" {
			finalPath = mapRestorePath(originalPath, fsr.RestoreDir)
		}

		err = fsr.restoreFile(finalPath, meta.Size, os.FileMode(meta.Mode))
		if err != nil {
			slog.Error("error restore file", slog.String("path", finalPath), slog.String("err", err.Error()))
			return err
		}
	}

	fsr.Source.Close()

	return nil
}

func (fsr *FileSystemRestore) restoreFile(restorePath string, fileSize int64, mode os.FileMode) error {
	dir := filepath.Dir(restorePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		slog.Error("error create directory", slog.String("directory", dir), slog.String("err", err.Error()))
		return err
	}

	f, err := os.OpenFile(restorePath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, mode)
	if err != nil {
		slog.Error("error create file", slog.String("file", restorePath), slog.String("err", err.Error()))
		return err
	}
	defer f.Close()
	slog.Info("create file", slog.String("path", restorePath))

	var written int64
	buffer := make([]byte, 64*1024)
	for {
		n, err := fsr.Source.ReadChunk(buffer)
		slog.Info("прочитано байт", slog.Int("count", n))
		if n > 0 {
			w, err := f.Write(buffer[:n])
			if err != nil {
				slog.Error("error write file", slog.String("file", restorePath), slog.String("err", err.Error()))
				return err
			}
			slog.Info("записано байт", slog.Int("count", n))
			written += int64(w)
		}
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return err
		}
	}

	if written != fileSize {
		return fmt.Errorf("файл %s записан не полностью: ожидалось %d, записано %d", restorePath, fileSize, written)
	}

	return nil
}

// Функция для корректировки нового пути восстановления
func mapRestorePath(originalPath string, targetDir string) string {
	cleanPath := filepath.Clean(originalPath)
	relPath, err := filepath.Rel("/", cleanPath)

	if err != nil {
		return filepath.Join(targetDir, cleanPath)
	}

	return filepath.Join(targetDir, relPath)
}
