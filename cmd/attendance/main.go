package main

import (
	"context"
	"errors"
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"auto-attendance/internal/config"
	"auto-attendance/internal/ethol"
	"auto-attendance/internal/logging"
	"auto-attendance/internal/scheduler"
	"auto-attendance/internal/telegram"
)

var Version = "dev"

func main() {
	configPath := flag.String("config", "config.yaml", "path to YAML configuration")
	once := flag.Bool("once", false, "jalankan satu kali pemeriksaan presensi lalu keluar")
	dryRun := flag.Bool("dry-run", false, "mode simulasi, cek presensi tanpa melakukan submit")
	versionFlag := flag.Bool("version", false, "tampilkan versi aplikasi lalu keluar")
	flag.Parse()

	if *versionFlag {
		log.Printf("auto-attendance %s\n", Version)
		return
	}

	cfg, err := config.Load(*configPath)
	if err != nil {
		log.Fatal(err)
	}

	logStore, err := logging.Configure(cfg.Timezone, "log.db", cfg.Logging.RetentionDays)
	if err != nil {
		log.Printf("peringatan: gagal membuka sqlite log store: %v", err)
	}
	if logStore != nil {
		defer logStore.Close()
	}

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
	log.Printf("auto-attendance %s dimulai", Version)
	log.Printf("login berhasil untuk %s (%d)", student.Name, student.Number)

	app := scheduler.New(client, cfg, student)
	if *dryRun {
		app.SetDryRun(true)
		log.Println("mode: DRY-RUN aktif (tidak ada presensi yang akan di-submit)")
	}

	// Initialize Telegram Bot & Notifier if configured
	if cfg.Telegram.Token != "" && cfg.Telegram.ChatID != "" {
		loc, _ := time.LoadLocation(cfg.Timezone)
		bot := telegram.NewBot(cfg.Telegram.Token, cfg.Telegram.ChatID, client, app, student, loc, Version)
		if bot != nil {
			notifier := telegram.NewNotifier(bot.Client(), loc)
			app.SetNotifier(notifier)
			bot.Start(ctx)
		}
	}

	if *once {
		if err := app.RunOnce(ctx); err != nil && !errors.Is(err, context.Canceled) {
			log.Print(err)
			os.Exit(1)
		}
		return
	}

	if err := app.Run(ctx); err != nil && !errors.Is(err, context.Canceled) {
		log.Print(err)
		os.Exit(1)
	}
	log.Println("aplikasi dihentikan dengan aman (graceful shutdown)")
}
