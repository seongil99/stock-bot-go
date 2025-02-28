package main

import (
	"errors"
	"fmt"
	"log"
	"math"
	"os"
	"strconv"
	"sync"
	"time"

	"stock-bot/models"
	"stock-bot/services"

	"github.com/joho/godotenv"
)

// Application constants
const (
	appName              = "Stock Price Bot"
	version              = "1.0.0"
	alertThreshold       = 5.0 // Alert threshold for price changes over 5%
	maxConcurrency       = 5   // Maximum number of concurrent requests
	checkInterval        = 15  // Scheduler check interval in minutes
	defaultCheckHour     = 7   // Default time for daily report (7AM)
	realtimeCheckMinutes = 30  // Interval for realtime price checks in minutes
)

// Environment variable keys
const (
	envMongoURI       = "MONGODB_URI"
	envTelegramToken  = "TELEGRAM_BOT_TOKEN"
	envTelegramChatID = "TELEGRAM_CHAT_ID"
	envLineToken      = "LINE_CHANNEL_ACCESS_TOKEN"
	envTimezone       = "TIMEZONE"
	envCheckHour      = "CHECK_HOUR"
)

// Global variable to track the last processed date
var lastProcessedDate string

// Map to track the last alert time for each stock
var lastAlertSentMap = make(map[string]time.Time)
var alertMapMutex sync.RWMutex

func main() {
	log.Printf("Starting %s v%s", appName, version)

	// Load environment variables
	config, err := loadConfig()
	if err != nil {
		log.Fatal("Configuration error: ", err)
	}

	// Connect to database
	db, err := services.NewDatabase(config.MongoURI)
	if err != nil {
		log.Fatal("Database connection error: ", err)
	}
	defer func() {
		if err := db.Close(); err != nil {
			log.Printf("Error closing database connection: %v", err)
		}
	}()
	log.Printf("Connected to database")

	// Initialize messenger
	messenger, err := initializeMessenger(config)
	if err != nil {
		log.Fatal("Messenger initialization error: ", err)
	}

	// Start scheduler
	runScheduler(db, messenger, config)
}

// loadConfig loads application settings from environment variables
func loadConfig() (models.Config, error) {
	// Load .env file
	if err := godotenv.Load(); err != nil {
		log.Printf("Warning: .env file not found, using environment variables")
	}

	config := models.DefaultConfig()

	// MongoDB URI
	config.MongoURI = os.Getenv(envMongoURI)
	if config.MongoURI == "" {
		return config, fmt.Errorf("required environment variable %s not set", envMongoURI)
	}

	// Telegram settings
	config.TelegramBotToken = os.Getenv(envTelegramToken)
	config.TelegramChatID = os.Getenv(envTelegramChatID)

	// Line settings
	config.LineChannelToken = os.Getenv(envLineToken)

	// Ensure at least one messaging service is configured
	if config.TelegramBotToken == "" && config.LineChannelToken == "" {
		return config, fmt.Errorf("at least one messaging service (Telegram or Line) must be configured")
	}

	// Timezone settings
	if tz := os.Getenv(envTimezone); tz != "" {
		config.TimeZone = tz
	}

	// Check hour settings
	if hourStr := os.Getenv(envCheckHour); hourStr != "" {
		if hour, err := strconv.Atoi(hourStr); err == nil && hour >= 0 && hour < 24 {
			config.CheckHour = hour
		} else {
			log.Printf("Warning: invalid %s value, using default: %d", envCheckHour, defaultCheckHour)
		}
	} else {
		// Set default value
		config.CheckHour = defaultCheckHour
	}

	return config, nil
}

// initializeMessenger initializes the messaging service
func initializeMessenger(config models.Config) (services.Messenger, error) {
	// Use Telegram messenger with priority
	if config.TelegramBotToken != "" && config.TelegramChatID != "" {
		return services.NewTelegramMessenger(config.TelegramBotToken, config.TelegramChatID)
	}

	// Use Line messenger
	if config.LineChannelToken != "" {
		return services.NewLineMessenger(config.LineChannelToken)
	}

	return nil, fmt.Errorf("no valid messenger configuration found")
}

// runScheduler executes the scheduling logic
func runScheduler(db *services.Database, messenger services.Messenger, config models.Config) {
	// Set timezone
	loc, err := time.LoadLocation(config.TimeZone)
	if err != nil {
		log.Printf("Warning: could not load timezone %s, using local timezone", config.TimeZone)
		loc = time.Local
	}
	log.Printf("Scheduler using timezone: %s", loc.String())

	// Start scheduler
	log.Printf("Starting scheduler with check interval of %d minutes", checkInterval)
	log.Printf("Will perform daily price reports at %d:00 (timezone: %s)", config.CheckHour, config.TimeZone)
	log.Printf("Will check for significant price changes every %d minutes", realtimeCheckMinutes)

	ticker := time.NewTicker(time.Duration(checkInterval) * time.Minute)
	defer ticker.Stop()

	// Check current time at initial run
	checkAndProcess(db, messenger, config, loc)

	// Periodic execution
	for range ticker.C {
		checkAndProcess(db, messenger, config, loc)
	}
}

// checkAndProcess checks the current time and runs the price collection process if needed
func checkAndProcess(db *services.Database, messenger services.Messenger, config models.Config, loc *time.Location) {
	now := time.Now().In(loc)
	currentDate := now.Format("2006-01-02")

	log.Printf("Checking time: %s", now.Format("2006-01-02 15:04:05"))

	// 1. Run daily report at specified time (7AM) if not already run today
	if now.Hour() == config.CheckHour && now.Minute() < checkInterval && lastProcessedDate != currentDate {
		log.Printf("Starting daily price report at scheduled time")
		sendDailyReport(db, messenger, config)

		// Record today's date
		lastProcessedDate = currentDate
		log.Printf("Daily report processed for date: %s", lastProcessedDate)

		// Reset alert map at the start of a new day
		resetAlertMap()
	}

	// 2. Periodic realtime price check (only during market hours)
	// Skip if market is closed
	if !isMarketOpen(now) {
		return
	}

	// Check at specified realtime intervals
	if now.Minute()%realtimeCheckMinutes == 0 {
		log.Printf("Checking for realtime price changes")
		checkRealtimePriceChanges(db, messenger, config)
	}
}

// isMarketOpen checks if the current time is during stock market hours
// US market hours: Mon-Fri, 9:30AM-4:00PM ET (Korean time 23:30-7:00)
func isMarketOpen(now time.Time) bool {
	// Exclude weekends
	if now.Weekday() == time.Saturday || now.Weekday() == time.Sunday {
		return false
	}

	// Time zone conversion may be needed (simplified implementation for now)
	hour := now.Hour()

	// Example: Assuming 23:30-07:00 Korean time as market hours
	return (hour >= 21 && hour <= 23) || (hour >= 0 && hour <= 7)
}

// resetAlertMap resets the alert tracking map at the start of a new day
func resetAlertMap() {
	alertMapMutex.Lock()
	defer alertMapMutex.Unlock()

	lastAlertSentMap = make(map[string]time.Time)
	log.Printf("Alert tracking map has been reset for new day")
}

// canSendAlert checks if an alert has already been sent today for a specific stock
func canSendAlert(symbol string) bool {
	alertMapMutex.RLock()
	defer alertMapMutex.RUnlock()

	lastSent, exists := lastAlertSentMap[symbol]
	if !exists {
		return true
	}

	// Check if the last alert was sent on a different date
	now := time.Now()
	return lastSent.Day() != now.Day() || lastSent.Month() != now.Month() || lastSent.Year() != now.Year()
}

// markAlertSent records that an alert has been sent for a specific stock
func markAlertSent(symbol string) {
	alertMapMutex.Lock()
	defer alertMapMutex.Unlock()

	lastAlertSentMap[symbol] = time.Now()
}

// sendDailyReport sends a daily price report for all stocks
func sendDailyReport(db *services.Database, messenger services.Messenger, config models.Config) {
	log.Printf("Fetching stock prices for daily report")

	// Fetch prices
	prices, err := fetchAllPrices(config)
	if err != nil {
		log.Printf("Error during price fetching for daily report: %v", err)
		return
	}

	// Send daily report
	if err := messenger.SendMessage(prices, nil); err != nil {
		log.Printf("Error sending daily price report: %v", err)
	} else {
		log.Printf("Daily price report sent successfully")
	}
}

// checkRealtimePriceChanges checks for significant price changes in real-time and sends alerts
func checkRealtimePriceChanges(db *services.Database, messenger services.Messenger, config models.Config) {
	// Fetch prices
	prices, err := fetchAllPrices(config)
	if err != nil {
		log.Printf("Error during price fetching for realtime check: %v", err)
		return
	}

	// Check for changes in each stock
	var alertsToSend []models.PriceAlert

	for symbol, priceStr := range prices {
		// Skip if an alert has already been sent today
		if !canSendAlert(symbol) {
			continue
		}

		// Check for significant changes
		alert, hasSignificantChange := checkPriceChange(db, symbol, priceStr)
		if !hasSignificantChange {
			continue
		}

		// Add alert
		alertsToSend = append(alertsToSend, alert)

		// Record that an alert has been sent
		markAlertSent(symbol)
		log.Printf("Significant price change detected for %s (%.2f%%)", symbol, alert.PercentChange)
	}

	// Send alerts only if there are any
	if len(alertsToSend) > 0 {
		log.Printf("Sending realtime alerts for %d stocks with significant changes", len(alertsToSend))

		if err := messenger.SendAlerts(alertsToSend, nil); err != nil {
			log.Printf("Error sending realtime price alerts: %v", err)
		} else {
			log.Printf("Realtime price alerts sent successfully")
		}
	}
}

// fetchAllPrices fetches prices for all stocks
func fetchAllPrices(config models.Config) (map[string]string, error) {
	priceFetcher := services.NewPriceFetcher()

	// Fetch price information
	priceResults, err := priceFetcher.FetchPriceConcurrent(models.Tickers, maxConcurrency)
	if err != nil {
		return nil, fmt.Errorf("error during price fetching: %w", err)
	}

	// Process results
	prices := make(map[string]string)
	var successCount int

	for symbol, result := range priceResults {
		if result.Error != nil {
			log.Printf("Error fetching price for %s: %v", symbol, result.Error)
			continue
		}

		prices[symbol] = result.Price
		successCount++
	}

	// If all price fetching failed
	if successCount == 0 {
		return nil, fmt.Errorf("failed to fetch any stock prices")
	}

	log.Printf("Successfully fetched %d/%d stock prices", successCount, len(models.Tickers))
	return prices, nil
}

// checkPriceChange checks for significant changes in stock prices
func checkPriceChange(db *services.Database, symbol, currentPriceStr string) (models.PriceAlert, bool) {
	// Parse current price
	currentPrice, err := strconv.ParseFloat(currentPriceStr, 64)
	if err != nil {
		log.Printf("Error parsing current price for %s: %v", symbol, err)
		return models.PriceAlert{}, false
	}

	// Get previous closing price
	previousPrice, err := db.GetLatestClosingPrice(symbol)
	if err != nil {
		if !errors.Is(err, services.ErrNoClosingPriceFound) {
			log.Printf("Error retrieving previous closing price for %s: %v", symbol, err)
		}
		return models.PriceAlert{}, false
	}

	// Skip if this is the first data point (no previous price)
	if previousPrice == 0 {
		return models.PriceAlert{}, false
	}

	// Calculate percentage change
	percentChange := ((currentPrice - previousPrice) / previousPrice) * 100

	// Create alert if change exceeds threshold
	if math.Abs(percentChange) >= alertThreshold {
		alert := models.PriceAlert{
			Symbol:        symbol,
			PreviousPrice: previousPrice,
			CurrentPrice:  currentPrice,
			PercentChange: percentChange,
			Timestamp:     time.Now(),
		}

		// Save current price to DB
		if err := db.SavePrice(symbol, currentPriceStr, false, nil); err != nil {
			log.Printf("Error saving current price data for %s: %v", symbol, err)
		}

		return alert, true
	}

	return models.PriceAlert{}, false
}
