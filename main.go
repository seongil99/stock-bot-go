package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/chromedp/chromedp"
	"github.com/google/uuid"
	"github.com/joho/godotenv"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

type PriceResult struct {
	Symbol string
	Price  string
}

type MongoDTO struct {
  Symbol    string
  Price     string  
  Timestamp time.Time
}

var tickers = []string {
  "AAPL",
  "GOOGL", 
  "AMZN",
  "MSFT", 
  "TSLA", 
  "NVDA", 
  "NFLX",
  "META",
}

func savePriceToMongoDB(symbol string, price string, wg *sync.WaitGroup) {
	defer wg.Done()

	mongoURI := os.Getenv("MONGODB_URI")
	if mongoURI == "" {
		log.Fatal("MONGODB_URI not set")
  }

	mongoClient, err := mongo.Connect(options.Client().ApplyURI(mongoURI))
  if err != nil {
		log.Fatal("MongoDB connection error: ", err)
	}
	defer func() {
		if err := mongoClient.Disconnect(context.Background()); err != nil {
			log.Fatal("MongoDB disconnect error: ", err)
		}
	}()

	collection := mongoClient.Database("stock_data").Collection("stocks")
	stockData := map[string]interface{}{
		"symbol":    symbol,
		"price":     price,
		"timestamp": time.Now(),
	}

	_, err = collection.InsertOne(context.Background(), stockData)
	if err != nil {
		log.Fatal("Failed to insert stock data: ", err)
	}

  // check data in MongoDB
  var result MongoDTO
  err = collection.FindOne(context.Background(), map[string]string{"symbol": symbol}).Decode(&result)
  if err != nil {
    log.Fatal("Failed to find stock data: ", err)
  }
  log.Printf("Found %s: %s in MongoDB", result.Symbol, result.Price)

  log.Printf("Saved %s: %s to MongoDB", symbol, price)
}

// fetchPrice navigates to the given URL and extracts the price from the page.
func fetchPrice(ctx context.Context, url string) (string, error) {
	var price string
	// Run the chromedp tasks.
  log.Printf("Fetching price from %s", url)
	err := chromedp.Run(ctx,
		chromedp.Navigate(url),
		chromedp.WaitVisible(`span[data-testid="qsp-price"]`, chromedp.ByQuery),
		chromedp.Text(`span[data-testid="qsp-price"]`, &price, chromedp.ByQuery),
	)
	if err != nil {
		return "", err
	}
	return price, nil
}

func postLineMessage(prices map[string]string, wg *sync.WaitGroup) {  
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

func postTelegramMessage(prices map[string]string, wg *sync.WaitGroup) {
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
    "text": message,
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

func main() {
	err := godotenv.Load()
	if err != nil {
		log.Fatal("Error loading .env file")
	}

  urls := make(map[string]string)
  for _, t := range tickers {
    urls[t] = fmt.Sprintf("https://finance.yahoo.com/quote/%s/", t)
  }

  opts := append(chromedp.DefaultExecAllocatorOptions[:],
    chromedp.DisableGPU,
    chromedp.NoDefaultBrowserCheck,
    chromedp.NoFirstRun,
    chromedp.Headless,
    chromedp.NoSandbox,
    chromedp.Flag("blink-settings", "scriptEnabled=false, imagesEnabled=false"),
    chromedp.Flag("disable-extensions", true),
  )

  // Create a shared ExecAllocator context with desired options.
  allocCtx, allocCancel := chromedp.NewExecAllocator(context.Background(), opts...)
  defer allocCancel()

  // Create a shared parent context from the allocator.
  parentCtx, parentCancel := chromedp.NewContext(allocCtx, chromedp.WithLogf(log.Printf))
  defer parentCancel()

  // Semaphore to limit cuncurrency.
  sem := make(chan struct{}, 5)
  var wg sync.WaitGroup
  var mu sync.Mutex
  results := make(map[string]string)


  // fetch prices every one minute 23:00 - 06:00
  // for {
  //   now := time.Now()
  //   if now.Hour() >= 23 || now.Hour() < 6 {
  //     break
  //   }
  //   time.Sleep(time.Minute)
  // }


  for ticker, url := range urls {
    wg.Add(1)
    go func(ticker, url string) {
      defer wg.Done()

      // Acquire a slot.
      sem <- struct{}{}

      // Create a new child context from the shared parent, with a timeout.
      childCtx, childCancel := chromedp.NewContext(parentCtx)
      defer childCancel()
      childCtx, childCancel = context.WithTimeout(childCtx, time.Minute * 3)
      defer childCancel()

      price, err := fetchPrice(childCtx, url)
      if err != nil {
        log.Printf("Error fetching price from %s for %s: %v", url, ticker, err)
        price = "error"
      }

      // Save the result to MongoDB.
      var save_price sync.WaitGroup
      save_price.Add(1)
      go savePriceToMongoDB(ticker, price, &save_price)
      save_price.Wait()

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
    fmt.Printf("%s: %s\n", ticker, price)
  }

	// var send_message sync.WaitGroup
	// send_message.Add(1)
  // // go postLineMessage(prices, &send_message)
  // go postTelegramMessage(prices, &send_message)
  // send_message.Wait()
}
