package main

import (
	"fmt"
	"net"
	"os"

	agent "github.com/nikron173/datasaver/internal/mediaagent"
	"github.com/nikron173/datasaver/internal/pb"
	"google.golang.org/grpc"
)

func main() {
	// TODO: вынести в конфигурацию
	// 1. Настраиваем параметры сервера
	port := ":50012"           // Порт, который будет слушать Media Agent
	storageDir := "../backups" // Папка на сервере, где будут копиться бэкапы

	fmt.Println("[MEDIA-SERVER] Инициализация Сервера Хранения...")

	// 2. Создаем провайдер локального дискового хранилища
	diskStorage, err := agent.NewLocalDiskStorage(storageDir)
	if err != nil {
		fmt.Printf("[MEDIA-SERVER] Не удалось инициализировать папку хранилища: %v\n", err)
		os.Exit(1)
	}

	// 3. Открываем сетевой TCP-порт для прослушивания
	listener, err := net.Listen("tcp", port)
	if err != nil {
		fmt.Printf("[MEDIA-SERVER] Не удалось открыть порт %s: %v\n", port, err)
		os.Exit(1)
	}

	// 4. Создаем базовый gRPC сервер
	grpcServer := grpc.NewServer()

	// 5. Создаем наш бизнес-сервер бэкапа и регистрируем его в gRPC
	backupServer := agent.NewMediaAgentServer(diskStorage)
	pb.RegisterMediaAgentServiceServer(grpcServer, backupServer)

	fmt.Printf("[MEDIA-SERVER] Успешно запущен! Слушает порт %s\n", port)
	fmt.Printf("[MEDIA-SERVER] Все входящие бэкапы будут сохраняться в: %s\n", storageDir)

	// 6. Запускаем бесконечный цикл обработки сетевых запросов
	if err := grpcServer.Serve(listener); err != nil {
		fmt.Printf("[MEDIA-SERVER] Критическая ошибка работы gRPC сервера: %v\n", err)
		os.Exit(1)
	}
}
