package bot

import (
	"fmt"
	"net/http"
	"net/url"
)

func SendMessage(token, chatID, text string) error {
	apiURL := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", token)
	
	// Параметры запроса
	params := url.Values{}
	params.Add("chat_id", chatID)
	params.Add("text", text)

	_, err := http.PostForm(apiURL, params)
	return err
}