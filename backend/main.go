package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"self-manager/bot"
	"self-manager/db"
	"strconv"
	"strings"
	"time"
)

// Функция-обработчик для заметок
func notesHandler(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		notes, err := db.GetAllNotes()
		if err != nil {
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
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		
		if err := db.AddNote(n.Content, n.Deadline, n.Priority); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		token := os.Getenv("TELEGRAM_TOKEN")
		chatID := os.Getenv("TELEGRAM_CHAT_ID")
		bot.SendMessage(token, chatID, "🆕 Новая заметка: " + n.Content)
		w.WriteHeader(http.StatusCreated)
	
	case http.MethodDelete:
		// Извлекаем ID из URL (например, /api/notes?id=1)
		idStr := r.URL.Query().Get("id")
		id, _ := strconv.Atoi(idStr)
		if err := db.DeleteNote(id); err != nil {
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
			Priority string `json:"priority"` // Добавили
		}
		if err := json.NewDecoder(r.Body).Decode(&updateData); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		
		if err := db.UpdateNote(updateData.ID, updateData.Content, updateData.Deadline, updateData.Status, updateData.Notified, updateData.Priority); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)

	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func startNotificationWorker() {
	loc, _ := time.LoadLocation("Europe/Moscow")
	ticker := time.NewTicker(10 * time.Second)

	go func() {
		for range ticker.C {
			log.Println("Воркер проверяет БД...")
			
			rows, err := db.DB.Query("SELECT id, content, deadline, priority FROM notes WHERE notified = 0 AND deadline IS NOT NULL")
			if err != nil { continue }
			
			now := time.Now().In(loc)
			var idsToNotify []int

			for rows.Next() {
				var id int
				var content, deadlineStr, priority string
				rows.Scan(&id, &content, &deadlineStr, &priority)

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
					case "high": emoji = "🔴"
					case "low":  emoji = "🟢"
					}
					
					msgText := fmt.Sprintf("%s *Напоминание (%s приоритет):*\n\n%s", emoji, strings.ToUpper(priority), content)
					if err := bot.SendMessageWithButtons(token, chatID, msgText, id); err == nil {
						idsToNotify = append(idsToNotify, id) 
					}
				}
			}
			rows.Close() 

			for _, id := range idsToNotify {
				_, err := db.DB.Exec("UPDATE notes SET notified = 1 WHERE id = ?", id)
				if err != nil {
					log.Printf("Ошибка при пометке ID=%d: %v", id, err)
				}
			}
		}
	}()
}

func main() {
	staticPath := "./frontend/build"

	// Инициализация БД
	if err := db.InitDB("./data/manager.db"); err != nil {
		log.Fatal(err)
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

	go startNotificationWorker()

	log.Println("Сервер запущен на :8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}