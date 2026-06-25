package handlers

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"os"
	"strconv"

	"self-manager/bot"
	"self-manager/db"
)

func NotesHandler(w http.ResponseWriter, r *http.Request) {
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
			Content     string `json:"content"`
			Description string `json:"description"`
			Deadline    string `json:"deadline"`
			Priority    string `json:"priority"`
		}
		if err := json.NewDecoder(r.Body).Decode(&n); err != nil {
			slog.Warn("Ошибка декодирования POST body", "err", err)
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		insertedID, err := db.AddNote(n.Content, n.Description, n.Deadline, n.Priority)
		if err != nil {
			slog.Error("Ошибка добавления заметки в БД", "err", err, "content", n.Content)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		token := os.Getenv("TELEGRAM_TOKEN")
		chatID := os.Getenv("TELEGRAM_CHAT_ID")
		if token != "" && chatID != "" {
			bot.SendMessage(token, chatID, "🆕 Новая заметка: "+n.Content)
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(map[string]int64{"id": insertedID})

	case http.MethodDelete:
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
			ID          int    `json:"id"`
			Content     string `json:"content"`
			Description string `json:"description"`
			Deadline    string `json:"deadline"`
			Status      string `json:"status"`
			Notified    bool   `json:"notified"`
			Priority    string `json:"priority"`
		}
		if err := json.NewDecoder(r.Body).Decode(&updateData); err != nil {
			slog.Warn("Ошибка декодирования PUT body", "err", err)
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		if err := db.UpdateNote(updateData.ID, updateData.Content, updateData.Description, updateData.Deadline, updateData.Status, updateData.Notified, updateData.Priority); err != nil {
			slog.Error("Ошибка обновления заметки", "err", err, "id", updateData.ID)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)

	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}
