package diskagent

import (
	"context"
	"fmt"
	"time"

	"github.com/nikron173/datasaver/internal/archive"
	"github.com/nikron173/datasaver/internal/pb"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type NetworkSink struct {
	conn   *grpc.ClientConn
	stream pb.MediaAgentService_StreamBackupClient
}

// NewNetworkSink создает gRPC-клиента
func NewNetworkSink(serverAddr string) (*NetworkSink, error) {
	// TODO: сделать проверку serverAddr
	dialOptions := []grpc.DialOption{
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	}
	conn, err := grpc.NewClient(serverAddr, dialOptions...)
	if err != nil {
		return nil, fmt.Errorf("ошибка инициализации gRPC клиента: %w", err)
	}

	// 2. Создаем gRPC клиент из сгенерированного protobuf-пакета
	grpcClient := pb.NewMediaAgentServiceClient(conn)

	// 3. Открываем сетевой стрим с ограничением по времени на установку связи (5 секунд).
	// Если сервер выключен, функция StreamBackup завершится ошибкой в течение 5 секунд.
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	conn.Connect()

	for {
		state := conn.GetState()
		if state.String() == "READY" {
			break
		}
		if ctx.Err() != nil {
			conn.Close()
			return nil, fmt.Errorf("media-сервер недоступен (таймаут подключения)")
		}
		time.Sleep(10 * time.Millisecond)
	}

	stream, err := grpcClient.StreamBackup(context.Background())
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("не удалось установить связь или открыть gRPC стрим: %w", err)
	}

	return &NetworkSink{stream: stream}, nil
}

func (ns *NetworkSink) WriteFileMetadata(meta archive.FileHeader, path string) error {
	// Упаковываем данные в Protobuf структуру
	chunk := &pb.BackupChunk{
		Payload: &pb.BackupChunk_Metadata{
			Metadata: &pb.FileMetadata{
				FilePath: path,
				Size:     meta.Size,
				Mode:     meta.Mode,
			},
		},
	}
	// Отправляем пакет в gRPC сеть
	return ns.stream.Send(chunk)
}

func (ns *NetworkSink) WriteChunk(data []byte) error {
	chunk := &pb.BackupChunk{
		Payload: &pb.BackupChunk_DataBlock{
			DataBlock: data,
		},
	}
	return ns.stream.Send(chunk)
}

func (ns *NetworkSink) Close() error {
	// Закрываем стрим отправки со стороны клиента и получаем финальный ответ сервера
	resp, err := ns.stream.CloseAndRecv()
	if err != nil {
		return fmt.Errorf("ошибка закрытия gRPC стрима: %w", err)
	}
	if !resp.Success {
		return fmt.Errorf("сервер вернул ошибку: %s", resp.ErrorMessage)
	}
	fmt.Printf("[NETWORK] Сервер успешно сохранил %d файлов (%d байт)\n", resp.TotalFiles, resp.TotalBytes)
	return nil
}
