package worker

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"time"

	"self-manager/bot"
	"self-manager/db"
)

func StartNotificationWorker(ctx context.Context) {
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

				rows, err := db.DB.Query("SELECT id, content, deadline, priority FROM notes WHERE notified = 0 AND deadline IS NOT NULL AND status != 'done'")
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
