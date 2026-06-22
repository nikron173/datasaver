package utils

import (
	"io/fs"
	"log/slog"
	"path/filepath"
)

// Функция для корректировки нового пути восстановления
func MapRestorePath(originalPath string, targetDir string) string {
	cleanPath := filepath.Clean(originalPath)
	relPath, err := filepath.Rel("/", cleanPath)

	if err != nil {
		return filepath.Join(targetDir, cleanPath)
	}

	return filepath.Join(targetDir, relPath)
}

// Функция для рекурсивного получния файлов,
// На вход принимает путь, с которого будет происходить обход
func GetFiles(currentPath string) ([]string, error) {
	absCurrentPath, err := filepath.Abs(currentPath)
	if err != nil {
		return nil, err
	}

	files := make([]string, 0)

	filepath.WalkDir(absCurrentPath, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			slog.Error("error get file", slog.String("path", path), slog.String("err", err.Error()))
			return nil
		}

		if d.IsDir() {
			return nil
		}
		files = append(files, path)

		return nil
	})

	return files, nil
}
