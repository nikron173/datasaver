package main

import (
	"log/slog"
	"time"

	"github.com/nikron173/diskagent/internal/services"
)

func main() {
	err := services.CreateBackup(".", "../backup.dpbak.zst")
	if err != nil {
		slog.Error("error create backup", slog.String("err", err.Error()))
		return
	}

	time.Sleep(time.Second * 2)

	err = services.Restore("../backup.dpbak.zst", "./restore")
	if err != nil {
		slog.Error("error restore", slog.String("err", err.Error()))
		return
	}
}
