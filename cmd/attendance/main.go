package main

import (
	"context"
	"errors"
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"

	"auto-attendance/internal/config"
	"auto-attendance/internal/ethol"
	"auto-attendance/internal/logging"
	"auto-attendance/internal/scheduler"
)

func main() {
	configPath := flag.String("config", "config.yaml", "path to YAML configuration")
	flag.Parse()

	cfg, err := config.Load(*configPath)
	if err != nil {
		log.Fatal(err)
	}
	logging.Configure(cfg.Timezone)

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	client, err := ethol.New(cfg)
	if err != nil {
		log.Fatal(err)
	}
	student, err := client.Login(ctx)
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("login berhasil untuk %s (%d)", student.Name, student.Number)

	app := scheduler.New(client, cfg, student)
	if err := app.Run(ctx); err != nil && !errors.Is(err, context.Canceled) {
		log.Print(err)
		os.Exit(1)
	}
	log.Println("aplikasi dihentikan dengan aman (graceful shutdown)")
}
