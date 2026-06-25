package main

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"self-manager/bot"
	"self-manager/db"
	"self-manager/handlers"
	"self-manager/worker"
)

func main() {
	// Настройка структурированного логирования
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	slog.SetDefault(logger)

	staticPath := "./frontend/build"

	// Инициализация базы данных
	if err := db.InitDB("./data/manager.db"); err != nil {
		slog.Error("Критическая ошибка инициализации БД", "err", err)
		os.Exit(1)
	}

	// Инициализация API Эндпоинтов из слоя Handlers
	http.HandleFunc("/api/ping", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	})
	http.HandleFunc("/api/notes", handlers.NotesHandler)
	http.HandleFunc("/api/backgrounds", handlers.BackgroundsHandler)

	// Раздача фронтенд-статики и роутинг для SPA
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		path := filepath.Join(staticPath, r.URL.Path)
		if info, err := os.Stat(path); err == nil && !info.IsDir() {
			http.ServeFile(w, r, path)
			return
		}
		http.ServeFile(w, r, filepath.Join(staticPath, "index.html"))
	})

	// Контекст для централизованного контроля жизненного цикла горутин
	rootCtx, cancelRootCtx := context.WithCancel(context.Background())
	defer cancelRootCtx()

	token := os.Getenv("TELEGRAM_TOKEN")

	// Старт слушателя инлайн-кнопок Telegram (Long-polling)
	if token != "" {
		bot.StartCallbackListener(token, handlers.HandleTelegramCallback)
	}

	// Старт тикера дедлайнов
	worker.StartNotificationWorker(rootCtx)

	// Конфигурация и запуск HTTP-сервера
	srv := &http.Server{
		Addr:    ":8080",
		Handler: nil,
	}

	go func() {
		slog.Info("Сервер успешно запущен на порту :8080")
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("Критическая ошибка HTTP-сервера", "err", err)
			os.Exit(1)
		}
	}()

	// Обработка сигналов Graceful Shutdown
	stopSignal := make(chan os.Signal, 1)
	signal.Notify(stopSignal, os.Interrupt, syscall.SIGTERM)
	<-stopSignal

	slog.Info("Получен сигнал завершения. Начинаем Graceful Shutdown...")

	// 1. Посылаем сигнал отмены фоновым процессам (тикеру нотификаций)
	cancelRootCtx()

	// 2. Даем HTTP-серверу 5 секунд на завершение обработки текущих запросов
	shutdownCtx, cancelShutdown := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancelShutdown()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		slog.Error("Ошибка при принудительной остановке HTTP-сервера", "err", err)
	} else {
		slog.Info("HTTP-сервер штатно остановил работу")
	}
}
