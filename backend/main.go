package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"self-manager/bot"
	"self-manager/db"
	"strconv"
	"strings"
	"syscall"
	"time"
)

// Функция-обработчик для заметок
func notesHandler(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		notes, err := db.GetAllNotes()
		if err != nil {
			slog.Error("Не удалось получить заметки", "err", err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(notes)

	case http.MethodPost:
		var n struct { 
			Content  string `json:"content"` 
			Deadline string `json:"deadline"` 
			Priority string `json:"priority"`
		}
		if err := json.NewDecoder(r.Body).Decode(&n); err != nil {
			slog.Warn("Ошибка декодирования POST body", "err", err)
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		
		if err := db.AddNote(n.Content, n.Deadline, n.Priority); err != nil {
			slog.Error("Ошибка добавления заметки в БД", "err", err, "content", n.Content)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		token := os.Getenv("TELEGRAM_TOKEN")
		chatID := os.Getenv("TELEGRAM_CHAT_ID")
		if token != "" && chatID != "" {
			bot.SendMessage(token, chatID, "🆕 Новая заметка: "+n.Content)
		}
		w.WriteHeader(http.StatusCreated)
	
	case http.MethodDelete:
		// Извлекаем ID из URL (например, /api/notes?id=1)
		idStr := r.URL.Query().Get("id")
		id, _ := strconv.Atoi(idStr)
		if err := db.DeleteNote(id); err != nil {
			slog.Error("Ошибка удаления заметки", "err", err, "id", id)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)

	case http.MethodPut:
		var updateData struct {
			ID       int    `json:"id"`
			Content  string `json:"content"`
			Deadline string `json:"deadline"`
			Status   string `json:"status"`
			Notified bool   `json:"notified"`
			Priority string `json:"priority"`
		}
		if err := json.NewDecoder(r.Body).Decode(&updateData); err != nil {
			slog.Warn("Ошибка декодирования PUT body", "err", err)
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		
		if err := db.UpdateNote(updateData.ID, updateData.Content, updateData.Deadline, updateData.Status, updateData.Notified, updateData.Priority); err != nil {
			slog.Error("Ошибка обновления заметки", "err", err, "id", updateData.ID)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)

	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func startNotificationWorker(ctx context.Context) {
	loc, _ := time.LoadLocation("Europe/Moscow")
	ticker := time.NewTicker(10 * time.Second)

	go func() {
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				slog.Info("Фоновый воркер уведомлений успешно остановлен")
				return
			case <-ticker.C:
				slog.Info("Воркер проверяет БД...")
			
				rows, err := db.DB.Query("SELECT id, content, deadline, priority FROM notes WHERE notified = 0 AND deadline IS NOT NULL AND status = 'todo'")
				if err != nil {
					slog.Error("Ошибка выполнения запроса в воркере", "err", err)
					continue
				}
			
			now := time.Now().In(loc)
			var idsToNotify []int

			for rows.Next() {
				var id int
				var content, deadlineStr, priority string
					if err := rows.Scan(&id, &content, &deadlineStr, &priority); err != nil {
						slog.Error("Ошибка сканирования строки в воркере", "err", err)
						continue
					}

				deadlineTime, err := time.Parse(time.RFC3339, deadlineStr)
				if err == nil {
					deadlineTime = deadlineTime.In(loc)
				}

				if err == nil && now.After(deadlineTime) {
					token := os.Getenv("TELEGRAM_TOKEN")
					chatID := os.Getenv("TELEGRAM_CHAT_ID")
					
					// Подбираем эмодзи под уровень важности
					emoji := "🟡"
					switch priority {
						case "high":
							emoji = "🔴"
						case "low":
							emoji = "🟢"
					}
					
					msgText := fmt.Sprintf("%s *Напоминание (%s приоритет):*\n\n%s", emoji, strings.ToUpper(priority), content)
					if err := bot.SendMessageWithButtons(token, chatID, msgText, id); err == nil {
						idsToNotify = append(idsToNotify, id) 
						} else {
							slog.Error("Ошибка отправки сообщения через ТГ бот", "err", err, "note_id", id)
					}
				}
			}
			rows.Close() 

			for _, id := range idsToNotify {
				_, err := db.DB.Exec("UPDATE notes SET notified = 1 WHERE id = ?", id)
				if err != nil {
						slog.Error("Ошибка при пометке notified=1 в БД", "err", err, "id", id)
					}
				}
			}
		}
	}()
}

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	slog.SetDefault(logger)

	staticPath := "./frontend/build"

	// Инициализация БД
	if err := db.InitDB("./data/manager.db"); err != nil {
		slog.Error("Критическая ошибка инициализации БД", "err", err)
		os.Exit(1)
	}

	// API Маршруты
	http.HandleFunc("/api/ping", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	})

	http.HandleFunc("/api/notes", notesHandler)

	// Статика и SPA
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		path := filepath.Join(staticPath, r.URL.Path)
		if info, err := os.Stat(path); err == nil && !info.IsDir() {
			http.ServeFile(w, r, path)
			return
		}
		http.ServeFile(w, r, filepath.Join(staticPath, "index.html"))
	})

	// Контекст для контроля жизненного цикла горутин
	rootCtx, cancelRootCtx := context.WithCancel(context.Background())
	defer cancelRootCtx()

	token := os.Getenv("TELEGRAM_TOKEN")
	
	// ЗАПУСКАЕМ СЛУШАТЕЛЬ КЛИКОВ ПО КНОПКАМ
	if token != "" {
		bot.StartCallbackListener(token, func(action string, noteID int, callbackID string, chatID int64, msgID int, originalText string) {
			note, err := db.GetNoteByID(noteID)
			if err != nil {
				bot.AnswerCallback(token, callbackID, "Задача уже удалена")
				return
			}

			var textStatus string
			
			if action == "done" {
				// Сохраняем оригинальный дедлайн в строку, если он есть
				var deadlineStr string
				if note.Deadline != nil {
					deadlineStr = note.Deadline.Format(time.RFC3339)
				}
				// Перемещаем в колонку "done" и оставляем notified = true
				db.UpdateNote(note.ID, note.Content, deadlineStr, "done", true, note.Priority)
				textStatus = "✅ Выполнено"
				
			} else if action == "postpone" {
				// Сдвигаем дедлайн на 1 час вперед относительно текущего момента
				loc, _ := time.LoadLocation("Europe/Moscow")
				newDeadline := time.Now().In(loc).Add(1 * time.Hour).Format(time.RFC3339)
				
				// Статус оставляем прежним, но сбрасываем notified на false, чтобы воркер сработал опять
				db.UpdateNote(note.ID, note.Content, newDeadline, note.Status, false, note.Priority)
				textStatus = "⏰ Отложено на 1 час"
			}

			// 1. Гасим состояние загрузки на кнопке
			bot.AnswerCallback(token, callbackID, textStatus)
			
			// 2. Схлопываем кнопки в ТГ, обновляя сообщение финальным статусом
			finalText := fmt.Sprintf("%s\n\nИтог: %s", originalText, textStatus)
			bot.EditMessageText(token, chatID, msgID, finalText)
		})
	}

	// Запуск воркера с контекстом отмены
	startNotificationWorker(rootCtx)

	// Настройка HTTP-сервера для Graceful Shutdown
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

	// Ожидаем системных сигналов завершения
	stopSignal := make(chan os.Signal, 1)
	signal.Notify(stopSignal, os.Interrupt, syscall.SIGTERM)
	<-stopSignal

	slog.Info("Получен сигнал завершения. Начинаем Graceful Shutdown...")
	
	// 1. Останавливаем фоновые воркеры
	cancelRootCtx()

	// 2. Даем серверу 5 секунд на завершение текущих сетевых запросов
	shutdownCtx, cancelShutdown := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancelShutdown()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		slog.Error("Ошибка при принудительной остановке HTTP-сервера", "err", err)
	} else {
		slog.Info("HTTP-сервер штатно остановил работу")
	}
}