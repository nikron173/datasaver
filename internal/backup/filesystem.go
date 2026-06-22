package backup

import (
	"encoding/binary"
	"fmt"
	"io"
	"log/slog"
	"os"

	"github.com/klauspost/compress/zstd"
	"github.com/nikron173/diskagent/internal/archive"
	"github.com/nikron173/diskagent/pkg/utils"
)

type FileSystemBackup struct {
	SrcPath     string
	ArchivePath string
}

func NewFileSystemBackup(srcPath string, archivePath string) *FileSystemBackup {
	return &FileSystemBackup{SrcPath: srcPath, ArchivePath: archivePath}
}

// Фунция создания бэкап-файла с использованием zstd,
// Принимает путь для бэкапа и где создать бэкап файл
func (fsb *FileSystemBackup) Run() error {
	bkpFile, err := os.Create(fsb.ArchivePath)
	if err != nil {
		slog.Error(
			"error create backup file",
			slog.String("path", fsb.ArchivePath),
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

	files, err := utils.GetFiles(fsb.SrcPath)
	if err != nil {
		slog.Error("error get files", slog.String("directory", fsb.SrcPath), slog.String("err", err.Error()))
		return err
	}

	for _, f := range files {
		err := fsb.backupFile(f, zstdWriter)
		if err != nil {
			slog.Error("error backup file", slog.String("path", f), slog.String("err", err.Error()))
			return err
		}
	}

	return nil
}

func (fsb *FileSystemBackup) backupFile(path string, writer io.Writer) error {
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
