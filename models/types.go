package models

import "time"

type PriceResult struct {
	Symbol string
	Price  string
}

type MongoDTO struct {
	Symbol    string
	Price     string
	Timestamp time.Time
}

var Tickers = []string{
	"AAPL",
	"GOOGL",
	"AMZN",
	"MSFT",
	"TSLA",
	"NVDA",
	"NFLX",
	"META",
}
