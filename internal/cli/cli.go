package cli

import (
	"flag"
	"fmt"
	"os"

	"github.com/nikron173/datasaver/internal/backup"
	"github.com/nikron173/datasaver/internal/restore"
)

// Execute парсит аргументы командной строки и запускает нужный модуль
func Execute() {
	// Инициализируем локальный FlagSet, чтобы не засорять глобальное пространство флагов
	fs := flag.NewFlagSet("datasaver", flag.ExitOnError)

	backupMode := fs.Bool("backup", false, "Запустить режим резервного копирования")
	restoreMode := fs.Bool("restore", false, "Запустить режим восстановления")

	src := fs.String("src", "", "Путь к папке (бэкап) или архиву (восстановление)")
	archivePath := fs.String("archive", "backup.dpbak.zst", "Путь к создаваемому файлу бэкапа")
	target := fs.String("target", "", "Альтернативная папка для восстановления")

	serverAddr := fs.String("server", "", "Адрес gRPC сервера Media Agent (например 192.168.1.11:50012). Если значение пустое - будет использоваться локальный режим")

	// Парсим аргументы, переданные приложению (исключая само имя бинарника os.Args[0])
	if err := fs.Parse(os.Args[1:]); err != nil {
		fmt.Printf("Ошибка парсинга флагов: %v\n", err)
		os.Exit(1)
	}

	// Валидация логики
	if *backupMode && *restoreMode {
		fmt.Println("Ошибка: Нельзя запускать бэкап и восстановление одновременно.")
		printUsage(fs)
		os.Exit(1)
	}

	if *backupMode {
		if *src == "" {
			fmt.Println("Ошибка: Укажите папку для бэкапа через флаг --src")
			os.Exit(1)
		}
		var sink backup.BackupSink
		var err error

		if *serverAddr != "" {
			fmt.Printf("[CLI] Выбран СЕТЕВОЙ режим. Подключение к Media Agent: %s...\n", *serverAddr)
			sink, err = backup.NewNetworkSink(*serverAddr)
			if err != nil {
				fmt.Printf("[CLI] Не удалось установить сетевое соединение: %v\n", err)
				os.Exit(1)
			}
		} else {
			fmt.Println("[CLI] Выбран ЛОКАЛЬНЫЙ режим бэкапа.")
			sink, err = backup.NewLocalSink(*archivePath)
			if err != nil {
				fmt.Printf("[CLI] Ошибка инициализации локального хранилища: %v\n", err)
				os.Exit(1)
			}
		}

		fsb := backup.NewFileSystemBackup(*src, sink)
		if err := fsb.Run(); err != nil {
			fmt.Printf("[BACKUP] Критическая ошибка: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("[BACKUP] Успешно завершено!")
		return
	}

	if *restoreMode {
		if *src == "" {
			fmt.Println("Ошибка: Укажите файл архива для восстановления через флаг --src")
			os.Exit(1)
		}
		fsr := restore.NewFileSystemRestore(*src, *target)
		if err := fsr.Run(); err != nil {
			fmt.Printf("[RESTORE] Критическая ошибка: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("[RESTORE] Успешно завершено!")
		return
	}

	// Если не выбран ни один режим
	printUsage(fs)
	os.Exit(1)
}

func printUsage(fs *flag.FlagSet) {
	fmt.Println("Использование datasaver:")
	fmt.Println("  Бэкап:          datasaver --backup --src /etc --archive my_etc.dpbak.zst")
	fmt.Println("  Сетевой бэкап:  datasaver --backup --src /etc --server 192.168.1.11:50012")
	fmt.Println("  Восстановление: datasaver --restore --src my_etc.dpbak.zst --target /tmp/recovered")
	fmt.Println("\nДоступные флаги:")
	fs.PrintDefaults()
}
