package main

import (
	"context"
	"log"
	"sync"
	"time"

	"stock-bot/models"
	"stock-bot/services"

	"github.com/chromedp/chromedp"
	"github.com/joho/godotenv"
)

func process() {
	priceFetcher := services.NewPriceFetcher()
	urls := services.GetURLs(models.Tickers)

	// Create a shared ExecAllocator context with desired options.
	allocCtx, allocCancel := chromedp.NewExecAllocator(context.Background(), priceFetcher.Opts...)
	defer allocCancel()

	// Create a shared parent context from the allocator.
	parentCtx, parentCancel := chromedp.NewContext(allocCtx, chromedp.WithLogf(log.Printf))
	defer parentCancel()

	// Semaphore to limit cuncurrency.
	sem := make(chan struct{}, 5)
	var wg sync.WaitGroup
	var mu sync.Mutex
	results := make(map[string]string)

	for ticker, url := range urls {
		wg.Add(1)
		go func(ticker, url string) {
			defer wg.Done()

			// Acquire a slot.
			sem <- struct{}{}

			// Create a new child context from the shared parent, with a timeout.
			childCtx, childCancel := chromedp.NewContext(parentCtx)
			defer childCancel()
			childCtx, childCancel = context.WithTimeout(childCtx, time.Minute*3)
			defer childCancel()

			price, err := priceFetcher.FetchPrice(childCtx, url)
			if err != nil {
				log.Printf("Error fetching price from %s for %s: %v", url, ticker, err)
				price = "error"
			}

			// Save the result safely.
			mu.Lock()
			results[ticker] = price
			mu.Unlock()

			// Release the slot.
			<-sem
		}(ticker, url)
	}

	wg.Wait()

	var prices = make(map[string]string)
	// Print the results.
	for ticker, price := range results {
		prices[ticker] = price
		log.Printf("%s: %s\n", ticker, price)
	}

	var send_message sync.WaitGroup
	send_message.Add(1)
	go (&services.TelegramMessenger{}).SendMessage(prices, &send_message)
	send_message.Wait()
}

func main() {
	log.Printf("Starting stock price checker")
	err := godotenv.Load()
	if err != nil {
		log.Fatal("Error loading .env file")
	}
	log.Printf("Loaded environment variables")

	loc, err := time.LoadLocation("Asia/Seoul")
	if err != nil {
		log.Fatal("Error loading location")
	}
	log.Printf("Loaded location")

	for {
		log.Printf("Checking time")
		now := time.Now().In(loc)
		if now.Hour() == 6 {
			log.Printf("Processing")
			process()
		}
		log.Printf("Sleeping")
		time.Sleep(time.Hour)
	}
}
