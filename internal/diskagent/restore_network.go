package diskagent

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"time"

	"github.com/nikron173/datasaver/internal/archive"
	"github.com/nikron173/datasaver/internal/pb"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type NetworkSource struct {
	conn           *grpc.ClientConn
	stream         pb.MediaAgentService_StreamRestoreClient
	remainingBytes int64
	dataBlock      []byte
}

// NewNetworkSink создает gRPC-клиента
func NewNetworkSource(serverAddr string, sessionID string) (*NetworkSource, error) {
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

	stream, err := grpcClient.StreamRestore(context.Background(), &pb.RestoreRequest{SessionId: sessionID})
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("не удалось установить связь или открыть gRPC стрим: %w", err)
	}

	return &NetworkSource{stream: stream}, nil
}

func (ns *NetworkSource) NextFile() (*archive.FileHeader, string, error) {
	if ns.remainingBytes > 0 {
		return nil, "", fmt.Errorf("the previous file has not been read yet")
	}
	restoreChunk, err := ns.stream.Recv()
	if err != nil {
		return nil, "", err
	}

	slog.Info("restore chunk", slog.String("type", restoreChunk.String()))

	restoreMeta, ok := restoreChunk.Payload.(*pb.RestoreChunk_Metadata)
	if !ok {
		return nil, "", fmt.Errorf("invalid file metadata")
	}
	meta := archive.NewFileHeader(
		int32(len(restoreMeta.Metadata.FilePath)),
		restoreMeta.Metadata.Size,
		restoreMeta.Metadata.Mode,
	)
	ns.remainingBytes = meta.Size

	return &meta, restoreMeta.Metadata.FilePath, nil
}

func (ns *NetworkSource) ReadChunk(buf []byte) (int, error) {
	if ns.remainingBytes <= 0 {
		return 0, io.EOF
	}

	if len(ns.dataBlock) == 0 {
		restoreChunk, err := ns.stream.Recv()
		if err != nil {
			return 0, err
		}

		dataChunk, ok := restoreChunk.Payload.(*pb.RestoreChunk_DataBlock)
		if !ok {
			return 0, fmt.Errorf("invalid data block")
		}
		ns.dataBlock = dataChunk.DataBlock
	}

	bytesToCopy := min(len(buf), len(ns.dataBlock))
	copy(buf, ns.dataBlock[:bytesToCopy])
	ns.remainingBytes -= int64(len(ns.dataBlock))
	ns.dataBlock = ns.dataBlock[bytesToCopy:]

	if ns.remainingBytes <= 0 {
		return bytesToCopy, io.EOF
	}

	return bytesToCopy, nil
}

func (ns *NetworkSource) Close() error {
	if ns.conn != nil {
		defer ns.conn.Close()
	}
	return ns.stream.CloseSend()
}
