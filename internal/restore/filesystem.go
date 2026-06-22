package restore

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/klauspost/compress/zstd"
	"github.com/nikron173/datasaver/internal/archive"
	"github.com/nikron173/datasaver/pkg/utils"
)

type FileSystemRestore struct {
	ArchivePath string
	RestoreDir  string
}

func NewFileSystemRestore(archivePath string, restoreDir string) *FileSystemRestore {
	return &FileSystemRestore{ArchivePath: archivePath, RestoreDir: restoreDir}
}

func (fsr *FileSystemRestore) Run() error {
	bkpFile, err := os.Open(fsr.ArchivePath)
	if err != nil {
		slog.Error(
			"error create backup file",
			slog.String("path", fsr.ArchivePath),
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
		var header archive.FileHeader

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
		if fsr.RestoreDir != "" {
			finalPath = utils.MapRestorePath(originalPath, fsr.RestoreDir)
		}

		err = fsr.restoreFile(finalPath, header.Size, os.FileMode(header.Mode), zstdReader)
		if err != nil {
			slog.Error("error restore file", slog.String("path", finalPath), slog.String("err", err.Error()))
			return err
		}
	}

	return nil
}

func (fsr *FileSystemRestore) restoreFile(targetPath string, fileSize int64, mode os.FileMode, reader io.Reader) error {
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
