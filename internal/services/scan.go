package services

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/klauspost/compress/zstd"
	"github.com/nikron173/diskagent/internal/models"
)

// Функция для рекурсивного получния файлов,
// На вход принимает путь, с которого будет происходить обход
func getFiles(currentPath string) ([]string, error) {
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

func backupFile(path string, writer io.Writer) error {
	f, err := os.Open(path)
	if err != nil {
		slog.Error("error open file", slog.String("path", path), slog.String("err", err.Error()))
		return err
	}
	defer f.Close()

	fStat, err := os.Stat(path)
	if err != nil {
		slog.Error("error get file stat", slog.String("path", path), slog.String("err", err.Error()))
		return err
	}

	header := models.NewFileHeader(
		int32(len(path)),
		fStat.Size(),
		uint32(fStat.Mode()),
	)

	// 3. Бинарная сериализация заголовка (Пишем строго определенные байты в writer)
	// binary.Write переводит числа в байты (используем BigEndian — стандарт сетевого порядка байт)
	if err := binary.Write(writer, binary.BigEndian, header.PathSize); err != nil {
		return err
	}
	if err := binary.Write(writer, binary.BigEndian, header.Size); err != nil {
		return err
	}
	if err := binary.Write(writer, binary.BigEndian, header.Mode); err != nil {
		return err
	}

	// 4. Пишем сам путь файла (строку переводим в байты)
	if _, err := writer.Write([]byte(path)); err != nil {
		return err
	}

	// 5. Потоково копируем тело файла в writer буферами по 32 КБ (без загрузки всего файла в RAM)
	// io.Copy делает это максимально эффективно на уровне ядра ОС
	written, err := io.Copy(writer, f)
	if err != nil {
		return err
	}

	if written != header.Size {
		return fmt.Errorf("размер файла изменился во время чтения: ожидалось %d, записано %d", header.Size, written)
	}

	return nil
}

// Фунция создания бэкап-файла с использованием zstd,
// Принимает путь для бэкапа и где создать бэкап файл
func CreateBackup(currentPath string, backupPath string) error {
	bkpFile, err := os.Create(backupPath)
	if err != nil {
		slog.Error(
			"error create backup file",
			slog.String("path", backupPath),
			slog.String("err", err.Error()),
		)
		return err
	}
	defer bkpFile.Close()

	zstdWriter, err := zstd.NewWriter(bkpFile)
	if err != nil {
		slog.Error("error initialization compression", slog.String("err", err.Error()))
		return err
	}
	defer zstdWriter.Close()

	files, err := getFiles(currentPath)
	if err != nil {
		slog.Error("error get files", slog.String("directory", currentPath), slog.String("err", err.Error()))
		return err
	}

	for _, f := range files {
		err := backupFile(f, zstdWriter)
		if err != nil {
			slog.Error("error backup file", slog.String("path", f), slog.String("err", err.Error()))
			return err
		}
	}

	return nil
}

func Restore(backupFile string, restoreDir string) error {
	bkpFile, err := os.Open(backupFile)
	if err != nil {
		slog.Error(
			"error create backup file",
			slog.String("path", backupFile),
			slog.String("err", err.Error()),
		)
		return err
	}
	defer bkpFile.Close()

	zstdReader, err := zstd.NewReader(bkpFile)
	if err != nil {
		slog.Error("error initialization decompressing", slog.String("err", err.Error()))
		return err
	}
	defer zstdReader.Close()

	for {
		var header models.FileHeader

		err := binary.Read(zstdReader, binary.BigEndian, &header.PathSize)
		if errors.Is(err, io.EOF) {
			break
		}

		if err != nil {
			slog.Error("error read path size", slog.String("err", err.Error()))
			return err
		}

		if err := binary.Read(zstdReader, binary.BigEndian, &header.Size); err != nil {
			slog.Error("error read file size", slog.String("err", err.Error()))
			return err
		}

		if err := binary.Read(zstdReader, binary.BigEndian, &header.Mode); err != nil {
			slog.Error("error read file mode", slog.String("err", err.Error()))
			return err
		}

		pathBuffer := make([]byte, header.PathSize)
		if _, err := io.ReadFull(zstdReader, pathBuffer); err != nil {
			slog.Error("error read path file", slog.String("err", err.Error()))
			return err
		}

		originalPath := string(pathBuffer)

		finalPath := originalPath
		if restoreDir != "" {
			finalPath = mapRestorePath(originalPath, restoreDir)
		}

		err = restoreFile(finalPath, header.Size, os.FileMode(header.Mode), zstdReader)
		if err != nil {
			slog.Error("error restore file", slog.String("path", finalPath), slog.String("err", err.Error()))
			return err
		}
	}

	return nil
}

func mapRestorePath(originalPath string, targetDir string) string {
	cleanPath := filepath.Clean(originalPath)
	relPath, err := filepath.Rel("/", cleanPath)

	if err != nil {
		return filepath.Join(targetDir, cleanPath)
	}

	return filepath.Join(targetDir, relPath)
}

func restoreFile(targetPath string, fileSize int64, mode os.FileMode, reader io.Reader) error {
	dir := filepath.Dir(targetPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		slog.Error("error create directory", slog.String("directory", dir), slog.String("err", err.Error()))
		return err
	}

	f, err := os.OpenFile(targetPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, mode)
	if err != nil {
		slog.Error("error create file", slog.String("file", targetPath), slog.String("err", err.Error()))
		return err
	}
	defer f.Close()

	// 3. Копируем ровно fileSize байт из общего потока в наш новый файл.
	// Использовать обычный io.Copy нельзя, так как он будет читать поток до самого конца архива.
	// io.LimitReader ограничивает чтение строго размером текущего файла.
	limitedReader := io.LimitReader(reader, fileSize)
	written, err := io.Copy(f, limitedReader)
	if err != nil {
		slog.Error("error write in file", slog.String("file", targetPath), slog.String("err", err.Error()))
		return err
	}

	if written != fileSize {
		return fmt.Errorf("файл %s записан не полностью: ожидалось %d, записано %d", targetPath, fileSize, written)
	}

	return nil
}
