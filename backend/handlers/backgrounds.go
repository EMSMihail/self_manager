package handlers

import (
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"time"
)

func BackgroundsHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	query := r.URL.Query().Get("query")
	if query == "" {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"results":[]}`))
		return
	}

	accessKey := os.Getenv("UNSPLASH_ACCESS_KEY")
	if accessKey == "" {
		slog.Error("Переменная окружения UNSPLASH_ACCESS_KEY не задана")
		http.Error(w, "Внутренняя ошибка конфигурации", http.StatusInternalServerError)
		return
	}

	escapedQuery := url.QueryEscape(query)
	unsplashURL := fmt.Sprintf("https://api.unsplash.com/search/photos?query=%s&per_page=12", escapedQuery)

	req, err := http.NewRequest(http.MethodGet, unsplashURL, nil)
	if err != nil {
		slog.Error("Ошибка создания запроса к Unsplash", "err", err)
		http.Error(w, "Внутренняя ошибка сервера", http.StatusInternalServerError)
		return
	}

	req.Header.Set("Authorization", "Client-ID "+accessKey)

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		slog.Error("Ошибка сети при запросе к Unsplash", "err", err)
		http.Error(w, "Ошибка внешнего API", http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(resp.StatusCode)

	_, err = io.Copy(w, resp.Body)
	if err != nil {
		slog.Error("Ошибка копирования тела ответа Unsplash", "err", err)
	}
}
