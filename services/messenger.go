package services

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"stock-bot/models"

	"github.com/google/uuid"
)

// Error definitions related to messaging
var (
	ErrTokenNotSet        = errors.New("messaging token not set")
	ErrChatIDNotSet       = errors.New("chat ID not set")
	ErrMessagePreparation = errors.New("failed to prepare message")
	ErrMessageSending     = errors.New("failed to send message")
)

// Messenger interface defines messaging services
type Messenger interface {
	SendMessage(prices map[string]string, wg *sync.WaitGroup) error
	SendAlerts(alerts []models.PriceAlert, wg *sync.WaitGroup) error
}

// LineMessenger implements Line messaging service
type LineMessenger struct {
	token string
}

// NewLineMessenger creates a new instance of LineMessenger
func NewLineMessenger(token string) (*LineMessenger, error) {
	if token == "" {
		return nil, ErrTokenNotSet
	}
	return &LineMessenger{token: token}, nil
}

// SendMessage sends stock price information via Line
func (lm *LineMessenger) SendMessage(prices map[string]string, wg *sync.WaitGroup) error {
	if wg != nil {
		defer wg.Done()
	}

	if lm.token == "" {
		return ErrTokenNotSet
	}

	retryKey := uuid.NewString()
	var message strings.Builder
	message.WriteString("📊 Daily Stock Report\n\n")

	for symbol, price := range prices {
		message.WriteString(fmt.Sprintf("%s: %s\n", symbol, price))
	}

	payload := map[string]interface{}{
		"messages": []map[string]string{
			{
				"type": "text",
				"text": message.String(),
			},
		},
	}

	jsonPayload, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrMessagePreparation, err)
	}

	client := &http.Client{Timeout: 10 * time.Second}
	req, err := http.NewRequest("POST", "https://api.line.me/v2/bot/message/broadcast", bytes.NewBuffer(jsonPayload))
	if err != nil {
		return fmt.Errorf("%w: %v", ErrMessagePreparation, err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", lm.token))
	req.Header.Set("X-Line-Retry-Key", retryKey)

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrMessageSending, err)
	}
	defer resp.Body.Close()

	log.Printf("LINE Bot push response: %s", resp.Status)

	if resp.StatusCode >= 400 {
		return fmt.Errorf("%w: received status code %d", ErrMessageSending, resp.StatusCode)
	}

	return nil
}

// SendAlerts sends stock price change alerts via Line
func (lm *LineMessenger) SendAlerts(alerts []models.PriceAlert, wg *sync.WaitGroup) error {
	if wg != nil {
		defer wg.Done()
	}

	if len(alerts) == 0 {
		return nil
	}

	if lm.token == "" {
		return ErrTokenNotSet
	}

	retryKey := uuid.NewString()
	var message strings.Builder
	message.WriteString("⚠️ Significant Price Changes Detected\n\n")

	for _, alert := range alerts {
		direction := "🔴 Decreased"
		if alert.PercentChange > 0 {
			direction = "🟢 Increased"
		}

		message.WriteString(fmt.Sprintf("%s: %s by %.2f%%\n",
			alert.Symbol,
			direction,
			alert.PercentChange,
		))
		message.WriteString(fmt.Sprintf("Previous: $%.2f → Current: $%.2f\n\n",
			alert.PreviousPrice,
			alert.CurrentPrice,
		))
	}

	payload := map[string]interface{}{
		"messages": []map[string]string{
			{
				"type": "text",
				"text": message.String(),
			},
		},
	}

	jsonPayload, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrMessagePreparation, err)
	}

	client := &http.Client{Timeout: 10 * time.Second}
	req, err := http.NewRequest("POST", "https://api.line.me/v2/bot/message/broadcast", bytes.NewBuffer(jsonPayload))
	if err != nil {
		return fmt.Errorf("%w: %v", ErrMessagePreparation, err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", lm.token))
	req.Header.Set("X-Line-Retry-Key", retryKey)

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrMessageSending, err)
	}
	defer resp.Body.Close()

	log.Printf("LINE Bot alert push response: %s", resp.Status)

	if resp.StatusCode >= 400 {
		return fmt.Errorf("%w: received status code %d", ErrMessageSending, resp.StatusCode)
	}

	return nil
}

// TelegramMessenger implements Telegram messaging service
type TelegramMessenger struct {
	token  string
	chatID string
}

// NewTelegramMessenger creates a new instance of TelegramMessenger
func NewTelegramMessenger(token, chatID string) (*TelegramMessenger, error) {
	if token == "" {
		return nil, ErrTokenNotSet
	}
	if chatID == "" {
		return nil, ErrChatIDNotSet
	}
	return &TelegramMessenger{token: token, chatID: chatID}, nil
}

// SendMessage sends stock price information via Telegram
func (tm *TelegramMessenger) SendMessage(prices map[string]string, wg *sync.WaitGroup) error {
	if wg != nil {
		defer wg.Done()
	}

	if tm.token == "" {
		return ErrTokenNotSet
	}
	if tm.chatID == "" {
		return ErrChatIDNotSet
	}

	var message strings.Builder
	message.WriteString("📊 *Daily Stock Report*\n\n")

	for symbol, price := range prices {
		message.WriteString(fmt.Sprintf("*%s*: %s\n", symbol, price))
	}

	return tm.sendTelegramMessage(message.String())
}

// SendAlerts sends stock price change alerts via Telegram
func (tm *TelegramMessenger) SendAlerts(alerts []models.PriceAlert, wg *sync.WaitGroup) error {
	if wg != nil {
		defer wg.Done()
	}

	if len(alerts) == 0 {
		return nil
	}

	if tm.token == "" {
		return ErrTokenNotSet
	}
	if tm.chatID == "" {
		return ErrChatIDNotSet
	}

	var message strings.Builder
	message.WriteString("⚠️ *Significant Price Changes Detected*\n\n")

	for _, alert := range alerts {
		direction := "🔴 Decreased"
		if alert.PercentChange > 0 {
			direction = "🟢 Increased"
		}

		message.WriteString(fmt.Sprintf("*%s*: %s by *%.2f%%*\n",
			alert.Symbol,
			direction,
			alert.PercentChange,
		))
		message.WriteString(fmt.Sprintf("  Previous: $%.2f → Current: $%.2f\n\n",
			alert.PreviousPrice,
			alert.CurrentPrice,
		))
	}

	return tm.sendTelegramMessage(message.String())
}

// sendTelegramMessage handles sending messages to Telegram
func (tm *TelegramMessenger) sendTelegramMessage(message string) error {
	payload := map[string]string{
		"chat_id":    tm.chatID,
		"text":       message,
		"parse_mode": "Markdown",
	}

	jsonPayload, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrMessagePreparation, err)
	}

	client := &http.Client{Timeout: 10 * time.Second}
	req, err := http.NewRequest("POST", fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", tm.token), bytes.NewBuffer(jsonPayload))
	if err != nil {
		return fmt.Errorf("%w: %v", ErrMessagePreparation, err)
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrMessageSending, err)
	}
	defer resp.Body.Close()

	log.Printf("Telegram Bot push response: %s", resp.Status)

	if resp.StatusCode >= 400 {
		return fmt.Errorf("%w: received status code %d", ErrMessageSending, resp.StatusCode)
	}

	return nil
}
