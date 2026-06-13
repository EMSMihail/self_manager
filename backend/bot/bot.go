package bot

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// SendMessage оствляем старым для совместимости (например, при создании новой заметки)
func SendMessage(token, chatID, text string) error {
	apiURL := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", token)
	
	params := url.Values{}
	params.Add("chat_id", chatID)
	params.Add("text", text)

	_, err := http.PostForm(apiURL, params)
	return err
}

// SendMessageWithButtons — НОВАЯ функция для воркера. Шлёт текст + инлайн-кнопки
func SendMessageWithButtons(token, chatID, text string, noteID int) error {
	apiURL := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", token)

	// Формируем JSON-строку разметки кнопок, зашивая туда ID нашей заметки
	replyMarkup := fmt.Sprintf(`{"inline_keyboard":[[{"text":"✅ Выполнено","callback_data":"done:%d"},{"text":"⏰ +1 час","callback_data":"postpone:%d"}]]}`, noteID, noteID)

	params := url.Values{}
	params.Add("chat_id", chatID)
	params.Add("text", text)
	params.Add("parse_mode", "Markdown")
	params.Add("reply_markup", replyMarkup)

	_, err := http.PostForm(apiURL, params)
	return err
}

// AnswerCallback гасит "часики" анимации загрузки на кнопке в Telegram
func AnswerCallback(token, callbackQueryID, text string) error {
	apiURL := fmt.Sprintf("https://api.telegram.org/bot%s/answerCallbackQuery", token)
	params := url.Values{}
	params.Add("callback_query_id", callbackQueryID)
	params.Add("text", text)

	_, err := http.PostForm(apiURL, params)
	return err
}

// EditMessageText заменяет текст сообщения (убирая кнопки) после успешного клика
func EditMessageText(token string, chatID int64, messageID int, text string) error {
	apiURL := fmt.Sprintf("https://api.telegram.org/bot%s/editMessageText", token)
	params := url.Values{}
	params.Add("chat_id", strconv.FormatInt(chatID, 10))
	params.Add("message_id", strconv.Itoa(messageID))
	params.Add("text", text)

	_, err := http.PostForm(apiURL, params)
	return err
}

// Нативные структуры для разбора ответов Telegram API
type TGUpdate struct {
	UpdateID      int `json:"update_id"`
	CallbackQuery *struct {
		ID      string `json:"id"`
		Data    string `json:"data"`
		Message *struct {
			MessageID int    `json:"message_id"`
			Text      string `json:"text"`
			Chat      struct {
				ID int64 `json:"id"`
			} `json:"chat"`
		} `json:"message"`
	} `json:"callback_query"`
}

type TGUpdatesResponse struct {
	Ok     bool       `json:"ok"`
	Result []TGUpdate `json:"result"`
}

// StartCallbackListener запускает бесконечный цикл long-polling обновлений от Telegram
func StartCallbackListener(token string, handleCallback func(action string, noteID int, callbackID string, chatID int64, msgID int, originalText string)) {
	offset := 0
	go func() {
		for {
			// Запрашиваем изменения, таймаут 30 секунд (long-polling)
			apiURL := fmt.Sprintf("https://api.telegram.org/bot%s/getUpdates?offset=%d&timeout=30", token, offset)
			resp, err := http.Get(apiURL)
			if err != nil {
				time.Sleep(5 * time.Second)
				continue
			}

			var updatesResp TGUpdatesResponse
			if err := json.NewDecoder(resp.Body).Decode(&updatesResp); err != nil {
				resp.Body.Close()
				time.Sleep(5 * time.Second)
				continue
			}
			resp.Body.Close()

			for _, update := range updatesResp.Result {
				// Сдвигаем offset, чтобы не получать старые события заново
				offset = update.UpdateID + 1

				// Если пришёл клик по inline-кнопке
				if update.CallbackQuery != nil && update.CallbackQuery.Message != nil {
					cb := update.CallbackQuery
					
					// Разбираем data (например: "done:15")
					parts := strings.Split(cb.Data, ":")
					if len(parts) == 2 {
						action := parts[0]
						noteID, _ := strconv.Atoi(parts[1])
						
						// Передаем управление в callback-функцию, которую мы опишем в main.go
						handleCallback(action, noteID, cb.ID, cb.Message.Chat.ID, cb.Message.MessageID, cb.Message.Text)
					}
				}
			}
		}
	}()
}