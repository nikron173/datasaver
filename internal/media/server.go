package media

import (
	"fmt"
	"io"
	"time"

	"github.com/klauspost/compress/zstd"
	"github.com/nikron173/datasaver/internal/archive"
	"github.com/nikron173/datasaver/internal/pb"
)

// BackupServer реализует gRPC службу BackupService
type BackupServer struct {
	pb.UnimplementedBackupServiceServer // Обязательно для совместимости с gRPC в Go
	Storage                             TargetStorage
}

func NewBackupServer(storage TargetStorage) *BackupServer {
	return &BackupServer{Storage: storage}
}

// StreamBackup обрабатывает входящий gRPC поток от Disk Agent
func (s *BackupServer) StreamBackup(stream pb.BackupService_StreamBackupServer) error {
	// 1. Генерируем ID сессии бэкапа (для MVP на основе временной метки)
	sessionID := fmt.Sprintf("session-%d", time.Now().UnixNano())
	fmt.Printf("[MEDIA-SERVER] Открыта новая сетевая сессия бэкапа: %s\n", sessionID)

	// 2. Открываем таргет в хранилище (получаем io.WriteCloser)
	storageWriter, err := s.Storage.OpenSession(sessionID)
	if err != nil {
		return fmt.Errorf("ошибка инициализации хранилища: %w", err)
	}
	defer storageWriter.Close()

	// 3. Накладываем ZSTD компрессию «на лету» на стороне сервера.
	// Сервер будет сжимать сырые чанки данных, приходящие по сети.
	zstdWriter, err := zstd.NewWriter(storageWriter)
	if err != nil {
		return fmt.Errorf("ошибка инициализации ZSTD на сервере: %w", err)
	}
	defer zstdWriter.Close()

	// Переменные для сбора статистики (понадобятся для финального ответа)
	var totalFiles int64
	var totalBytes int64

	// Вспомогательный кастомный упаковщик метаданных (аналогичный нашему старому локальному формату),
	// чтобы Media Agent записывал данные на диск в нашей бинарной спецификации.
	// Это позволит локальной утилите (dp --restore) читать эти файлы напрямую с диска сервера!
	archiveWriter := archive.NewArchiveWriter(zstdWriter)

	// 4. Запускаем цикл чтения чанков из сети gRPC
	for {
		chunk, err := stream.Recv()
		if err == io.EOF {
			// Клиент успешно завершил передачу данных
			break
		}
		if err != nil {
			fmt.Printf("[MEDIA-SERVER] Ошибка чтения из gRPC стрима: %v\n", err)
			return err
		}

		// Разбираем тип пакета с помощью конструкции switch на payload oneof
		switch payload := chunk.Payload.(type) {
		case *pb.BackupChunk_Metadata:
			meta := payload.Metadata
			totalFiles++
			fmt.Printf("[MEDIA-SERVER] Получен файл: %s (%d байт)\n", meta.FilePath, meta.Size)

			// Записываем бинарный заголовок в ZSTD архив сервера
			if err := archiveWriter.WriteFileMetadata(meta.FilePath, meta.Size, meta.Mode); err != nil {
				return fmt.Errorf("ошибка записи метаданных в архив: %w", err)
			}

		case *pb.BackupChunk_DataBlock:
			data := payload.DataBlock
			totalBytes += int64(len(data))

			// Записываем кусок сырых данных файла в ZSTD архив сервера
			if err := archiveWriter.WriteDataBlock(data); err != nil {
				return fmt.Errorf("ошибка записи блока данных в архив: %w", err)
			}
		}
	}

	// Важно принудительно сбросить буферы ZSTD перед отправкой успешного ответа клиенту
	archiveWriter.Close()

	fmt.Printf("[MEDIA-SERVER] Сессия %s успешно завершена. Сохранено файлов: %d, всего байт: %d\n",
		sessionID, totalFiles, totalBytes)

	// 5. Отправляем клиенту финальный отчет о проделанной работе
	response := &pb.BackupResponse{
		Success:    true,
		TotalFiles: totalFiles,
		TotalBytes: totalBytes,
	}

	return stream.SendAndClose(response)
}
