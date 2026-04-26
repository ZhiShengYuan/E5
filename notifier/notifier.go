package notifier

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
)

type Notifier interface {
	Notify(message string) error
}

type TelegramNotifier struct {
	botToken string
	chatID   string
	enabled  bool
}

func NewTelegramNotifier(botToken, chatID string) *TelegramNotifier {
	return &TelegramNotifier{
		botToken: botToken,
		chatID:   chatID,
		enabled:  botToken != "" && chatID != "",
	}
}

func (t *TelegramNotifier) Notify(message string) error {
	if !t.enabled {
		return nil
	}

	url := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", t.botToken)
	payload := map[string]string{
		"chat_id":    t.chatID,
		"text":       message,
		"parse_mode": "HTML",
	}

	body, _ := json.Marshal(payload)
	resp, err := http.Post(url, "application/json", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("telegram notify failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("telegram notify returned status %d", resp.StatusCode)
	}
	return nil
}

type NoOpNotifier struct{}

func (n *NoOpNotifier) Notify(message string) error { return nil }
