package models

import (
	"time"
)

// PriceResult contains stock symbol and price information
type PriceResult struct {
	Symbol string `json:"symbol"`
	Price  string `json:"price"`
	Error  error  `json:"-"` // Used when an error occurs
}

// MongoDTO is a structure for price information to be stored in MongoDB
type MongoDTO struct {
	Symbol    string    `bson:"symbol"`
	Price     string    `bson:"price"`
	Timestamp time.Time `bson:"timestamp"`
	IsClosing bool      `bson:"isClosing"`
}

// PriceAlert is a structure for price change notifications
type PriceAlert struct {
	Symbol        string    `json:"symbol"`
	PreviousPrice float64   `json:"previousPrice"`
	CurrentPrice  float64   `json:"currentPrice"`
	PercentChange float64   `json:"percentChange"`
	Timestamp     time.Time `json:"timestamp"`
}

// Ticker constants
const (
	AAPL  = "AAPL"
	GOOGL = "GOOGL"
	AMZN  = "AMZN"
	MSFT  = "MSFT"
	TSLA  = "TSLA"
	NVDA  = "NVDA"
	NFLX  = "NFLX"
	META  = "META"
)

// Tickers is a list of stock symbols to monitor
var Tickers = []string{
	AAPL,
	GOOGL,
	AMZN,
	MSFT,
	TSLA,
	NVDA,
	NFLX,
	META,
}

// Config manages application settings
type Config struct {
	MongoURI            string        `json:"mongoUri"`
	TelegramBotToken    string        `json:"telegramBotToken"`
	TelegramChatID      string        `json:"telegramChatId"`
	LineChannelToken    string        `json:"lineChannelToken"`
	CheckInterval       time.Duration `json:"checkInterval"`
	FetchTimeout        time.Duration `json:"fetchTimeout"`
	MaxConcurrency      int           `json:"maxConcurrency"`
	PriceAlertThreshold float64       `json:"priceAlertThreshold"`
	TimeZone            string        `json:"timeZone"`
	CheckHour           int           `json:"checkHour"`
}

// DefaultConfig returns default configuration values
func DefaultConfig() Config {
	return Config{
		CheckInterval:       15 * time.Minute,
		FetchTimeout:        2 * time.Minute,
		MaxConcurrency:      5,
		PriceAlertThreshold: 5.0,
		TimeZone:            "Asia/Seoul",
		CheckHour:           7,
	}
}
