package main

import (
	"encoding/json"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"self-manager/bot"
	"self-manager/db"
	"strconv"
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
		}
		if err := json.NewDecoder(r.Body).Decode(&n); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		
		// 1. Сохраняем в БД
		if err := db.AddNote(n.Content, n.Deadline); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		// 2. Отправляем уведомление в Telegram (замените на ваши данные)
		token := os.Getenv("TELEGRAM_TOKEN")
		chatID := os.Getenv("TELEGRAM_CHAT_ID")
		bot.SendMessage(token, chatID, "Новая заметка: " + n.Content)

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

	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func startNotificationWorker() {
    loc, _ := time.LoadLocation("Europe/Moscow")
    ticker := time.NewTicker(1 * time.Minute)

    go func() {
        for range ticker.C {
            log.Println("Воркер проверяет БД...")
            
            // 1. Сначала просто читаем, кто требует уведомления
            rows, err := db.DB.Query("SELECT id, content, deadline FROM notes WHERE notified = 0 AND deadline IS NOT NULL")
            if err != nil { continue }
            
            now := time.Now().In(loc)
            var idsToNotify []int

            for rows.Next() {
                var id int
                var content, deadlineStr string
                rows.Scan(&id, &content, &deadlineStr)

                deadlineTime, err := time.ParseInLocation("2006-01-02T15:04:05Z", deadlineStr, loc)
                if err != nil {
                    deadlineTime, err = time.ParseInLocation("2006-01-02T15:04", deadlineStr, loc)
                }

                if err == nil && now.After(deadlineTime) {
                    // Отправляем
                    token := os.Getenv("TELEGRAM_TOKEN")
                    chatID := os.Getenv("TELEGRAM_CHAT_ID")
                    if err := bot.SendMessage(token, chatID, "Напоминание: " + content); err == nil {
                        idsToNotify = append(idsToNotify, id) // Собираем ID в список
                    }
                }
            }
            rows.Close() // Обязательно закрываем курсор ПЕРЕД обновлением!

            // 2. Теперь, когда курсор закрыт, выполняем обновление
            for _, id := range idsToNotify {
                _, err := db.DB.Exec("UPDATE notes SET notified = 1 WHERE id = ?", id)
                if err != nil {
                    log.Printf("Ошибка при пометке ID=%d: %v", id, err)
                } else {
                    log.Printf("УСПЕХ: Запись ID=%d помечена как notified", id)
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

	go startNotificationWorker()

	log.Println("Сервер запущен на :8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}