package backup

import (
	"io"
	"log/slog"
	"os"

	"github.com/nikron173/datasaver/internal/archive"
	"github.com/nikron173/datasaver/pkg/utils"
)

// BackupSink определяет контракт для отправки данных бэкапа (на диск или в сеть)
type BackupSink interface {
	// WriteFileMetadata отправляет заголовки/метаданные файла
	WriteFileMetadata(meta archive.FileHeader, path string) error
	// WriteChunk отправляет кусок (именно кусок, чанк!) содержимого файла
	WriteChunk(data []byte) error
	// Close финализирует сессию (закрывает файл или сетевой стрим)
	Close() error
}

type FileSystemBackup struct {
	SrcPath string
	Sink    BackupSink
}

func NewFileSystemBackup(srcPath string, sink BackupSink) *FileSystemBackup {
	return &FileSystemBackup{SrcPath: srcPath, Sink: sink}
}

// Фунция создания бэкап-файла с использованием zstd,
// Принимает путь для бэкапа и где создать бэкап файл
func (fsb *FileSystemBackup) Run() error {
	// bkpFile, err := os.Create(fsb.ArchivePath)
	// if err != nil {
	// 	slog.Error(
	// 		"error create backup file",
	// 		slog.String("path", fsb.ArchivePath),
	// 		slog.String("err", err.Error()),
	// 	)
	// 	return err
	// }
	// defer bkpFile.Close()

	// zstdWriter, err := zstd.NewWriter(bkpFile)
	// if err != nil {
	// 	slog.Error("error initialization compression", slog.String("err", err.Error()))
	// 	return err
	// }
	// defer zstdWriter.Close()

	files, err := utils.GetFiles(fsb.SrcPath)
	if err != nil {
		slog.Error("error get files", slog.String("directory", fsb.SrcPath), slog.String("err", err.Error()))
		return err
	}

	for _, f := range files {
		err := fsb.backupFile(f)
		if err != nil {
			slog.Error("error backup file", slog.String("path", f), slog.String("err", err.Error()))
			return err
		}
	}

	if err := fsb.Sink.Close(); err != nil {
		slog.Error("error close backup sink", slog.String("err", err.Error()))
	}

	return nil
}

func (fsb *FileSystemBackup) backupFile(path string) error {
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

	header := archive.NewFileHeader(
		int32(len(path)),
		fStat.Size(),
		uint32(fStat.Mode()),
	)

	// 1. Отправляем метаданные в Sink
	if err := fsb.Sink.WriteFileMetadata(header, path); err != nil {
		slog.Error("error write file metadata", slog.String("path", path), slog.String("err", err.Error()))
		return err
	}

	// 2. Потоковое чтение файла фиксированными чанками по 64 КБ (Буфер памяти)
	// TODO: сделать какой-то конфиг для редактирования размера буффера
	buffer := make([]byte, 64*1024)
	for {
		n, err := f.Read(buffer)
		if n > 0 {
			// Отправляем в Sink ровно столько байт, сколько прочитали из файла
			if writeErr := fsb.Sink.WriteChunk(buffer[:n]); writeErr != nil {
				return writeErr
			}
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
	}

	slog.Info("обработан файл", slog.String("path", path), slog.Int64("bytes", header.Size))
	return nil
}
