package handlers

import (
	"fmt"
	"log/slog"
	"os"
	"time"

	"self-manager/bot"
	"self-manager/db"
)

func HandleTelegramCallback(action string, noteID int, callbackID string, chatID int64, msgID int, originalText string) {
	token := os.Getenv("TELEGRAM_TOKEN")
	note, err := db.GetNoteByID(noteID)
	if err != nil {
		bot.AnswerCallback(token, callbackID, "Задача уже удалена")
		return
	}

	var textStatus string

	if action == "done" {
		var deadlineStr string
		if note.Deadline != nil {
			deadlineStr = note.Deadline.Format(time.RFC3339)
		}

		db.UpdateNote(note.ID, note.Content, note.Description, deadlineStr, "done", true, note.Priority)
		textStatus = "✅ Выполнено"

	} else if action == "postpone" {
		loc, _ := time.LoadLocation("Europe/Moscow")
		newDeadline := time.Now().In(loc).Add(1 * time.Hour).Format(time.RFC3339)

		db.UpdateNote(note.ID, note.Content, note.Description, newDeadline, note.Status, false, note.Priority)
		textStatus = "⏰ Отложено на 1 час"
	}

	// 1. Гасим состояние загрузки на кнопке
	bot.AnswerCallback(token, callbackID, textStatus)

	// 2. Схлопываем кнопки в ТГ, обновляя сообщение финальным статусом
	finalText := fmt.Sprintf("%s\n\nИтог: %s", originalText, textStatus)
	if err := bot.EditMessageText(token, chatID, msgID, finalText); err != nil {
		slog.Error("Ошибка обновления текста сообщения в ТГ", "err", err, "msg_id", msgID)
	}
}
