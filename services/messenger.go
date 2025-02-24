package services

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"sync"

	"github.com/google/uuid"
)

type Messenger interface {
	SendMessage(prices map[string]string, wg *sync.WaitGroup)
}

type LineMessenger struct{}

func (lm *LineMessenger) SendMessage(prices map[string]string, wg *sync.WaitGroup) {
	defer wg.Done()

	token := os.Getenv("LINE_CHANNEL_ACCESS_TOKEN")
	if token == "" {
		log.Fatal("LINE_CHANNEL_ACCESS_TOKEN not set")
	}

	retryKey := uuid.NewString()
	var message string
	for symbol, price := range prices {
		message += fmt.Sprintf("%s: %s\n", symbol, price)
	}

	payload := map[string]interface{}{
		"messages": []map[string]string{
			{
				"type": "text",
				"text": message,
			},
		},
	}
	jsonPayload, err := json.Marshal(payload)
	if err != nil {
		log.Fatal("Failed to marshal JSON payload: ", err)
	}

	req, err := http.NewRequest("POST", "https://api.line.me/v2/bot/message/broadcast", bytes.NewBuffer(jsonPayload))
	if err != nil {
		log.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token))
	req.Header.Set("X-Line-Retry-Key", retryKey)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		log.Fatal(err)
	}
	defer resp.Body.Close()

	log.Printf("LINE Bot push response: %s", resp.Status)
}

type TelegramMessenger struct{}

func (tm *TelegramMessenger) SendMessage(prices map[string]string, wg *sync.WaitGroup) {
	defer wg.Done()

	token := os.Getenv("TELEGRAM_BOT_TOKEN")
	if token == "" {
		log.Fatal("TELEGRAM_BOT_TOKEN not set")
	}

	chatID := os.Getenv("TELEGRAM_CHAT_ID")
	if chatID == "" {
		log.Fatal("TELEGRAM_CHAT_ID not set")
	}

	var message string
	for symbol, price := range prices {
		message += fmt.Sprintf("%s: %s\n", symbol, price)
	}

	payload := map[string]string{
		"chat_id": chatID,
		"text":    message,
	}

	jsonPayload, err := json.Marshal(payload)
	if err != nil {
		log.Fatal("Failed to marshal JSON payload: ", err)
	}

	req, err := http.NewRequest("POST", fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", token), bytes.NewBuffer(jsonPayload))
	if err != nil {
		log.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		log.Fatal(err)
	}
	defer resp.Body.Close()

	log.Printf("Telegram Bot push response: %s", resp.Status)
}
