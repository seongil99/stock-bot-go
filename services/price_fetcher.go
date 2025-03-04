package services

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"stock-bot/models"

	"github.com/chromedp/chromedp"
)

// Error definitions for price fetching
var (
	ErrPriceFetchFailed = errors.New("failed to fetch price")
	ErrElementNotFound  = errors.New("price element not found")
	ErrBrowserTimeout   = errors.New("browser operation timed out")
)

// Global allocator and browser context to reuse across requests
var (
	globalAllocCtx      context.Context
	globalAllocCancel   context.CancelFunc
	globalBrowserCtx    context.Context
	globalBrowserCancel context.CancelFunc
	setupOnce           sync.Once
	cleanupOnce         sync.Once
	browserMutex        sync.Mutex
)

// PriceFetcher collects stock price information
type PriceFetcher struct {
	Opts          []chromedp.ExecAllocatorOption
	FetchTimeout  time.Duration
	MaxRetries    int
	RetryInterval time.Duration
}

// setupGlobalBrowser initializes the global browser instance
func setupGlobalBrowser() {
	// Create allocator context
	opts := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.DisableGPU,
		chromedp.NoDefaultBrowserCheck,
		chromedp.NoFirstRun,
		chromedp.Headless,
		chromedp.NoSandbox,
		chromedp.Flag("blink-settings", "scriptEnabled=false, imagesEnabled=false"),
		chromedp.Flag("disable-extensions", true),
		chromedp.Flag("disable-dev-shm-usage", true),
		chromedp.Flag("disable-setuid-sandbox", true),
		chromedp.Flag("no-zygote", true),
		chromedp.UserAgent("AppleWebKit/537.36 (KHTML, like Gecko) Chrome/64.0.3282.140 Safari/537.36"),
		chromedp.Flag("ignore-certificate-errors", true),
		chromedp.Flag("disable-features", "IsolateOrigins,site-per-process"),
		chromedp.Flag("disable-web-security", true),
		chromedp.Flag("no-default-browser-check", true),
	)

	// Create a background context for the allocator
	globalAllocCtx, globalAllocCancel = chromedp.NewExecAllocator(context.Background(), opts...)

	// Create a browser context
	globalBrowserCtx, globalBrowserCancel = chromedp.NewContext(
		globalAllocCtx,
		chromedp.WithLogf(log.Printf),
	)

	// Start the browser
	if err := chromedp.Run(globalBrowserCtx); err != nil {
		log.Printf("Error starting browser: %v", err)
	}

	// Set up signal handling for cleanup
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-c
		log.Println("Received termination signal, cleaning up browser")
		cleanupGlobalBrowser()
		os.Exit(0)
	}()
}

// cleanupGlobalBrowser properly closes the browser to prevent zombie processes
func cleanupGlobalBrowser() {
	cleanupOnce.Do(func() {
		log.Println("Cleaning up global browser")
		if globalBrowserCancel != nil {
			globalBrowserCancel()
		}
		if globalAllocCancel != nil {
			globalAllocCancel()
		}
	})
}

// NewPriceFetcher creates a new PriceFetcher instance
func NewPriceFetcher() *PriceFetcher {
	// Initialize the global browser if it hasn't been done yet
	setupOnce.Do(setupGlobalBrowser)

	return &PriceFetcher{
		FetchTimeout:  2 * time.Minute,
		MaxRetries:    3,
		RetryInterval: 5 * time.Second,
	}
}

// FetchPrice extracts stock price from a given URL
func (pf *PriceFetcher) FetchPrice(ctx context.Context, url string) (string, error) {
	var price string
	var err error
	log.Printf("Fetching price from %s", url)

	// Add retry logic
	for attempt := 0; attempt < pf.MaxRetries; attempt++ {
		if attempt > 0 {
			log.Printf("Retry attempt %d for %s", attempt, url)
			time.Sleep(pf.RetryInterval)
		}

		// Create a new tab context from the global browser context
		browserMutex.Lock()
		tabCtx, tabCancel := chromedp.NewContext(globalBrowserCtx)
		browserMutex.Unlock()

		// Add timeout to the tab context
		tabTimeoutCtx, cancel := context.WithTimeout(tabCtx, pf.FetchTimeout)

		// Always cancel the contexts when done with this iteration
		defer func() {
			cancel()
			tabCancel()
		}()

		// Execute the actions in the tab with timeout
		err = chromedp.Run(tabTimeoutCtx,
			chromedp.Navigate(url),
			chromedp.WaitVisible(`span[data-testid="qsp-price"]`, chromedp.ByQuery),
			chromedp.Text(`span[data-testid="qsp-price"]`, &price, chromedp.ByQuery),
		)

		// Return immediately on success
		if err == nil {
			return price, nil
		}

		// Retry on context cancellation/timeout
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			log.Printf("Browser operation timed out for %s, retrying...", url)
			continue
		}

		// Log other errors and retry
		log.Printf("Error fetching price from %s: %v", url, err)
	}

	// If all retries fail
	if err != nil {
		return "", fmt.Errorf("%w: %v", ErrPriceFetchFailed, err)
	}

	// If price was not found
	if price == "" {
		return "", ErrElementNotFound
	}

	return price, nil
}

// FetchPriceConcurrent fetches prices for multiple stocks concurrently
func (pf *PriceFetcher) FetchPriceConcurrent(ctx context.Context, tickers []string, maxConcurrency int) (map[string]models.PriceResult, error) {
	// Semaphore to limit concurrency
	sem := make(chan struct{}, maxConcurrency)

	// Results channel
	results := make(chan models.PriceResult, len(tickers))

	// waitgroup
	var wg sync.WaitGroup

	// Create URL mapping
	urls := GetURLs(tickers)

	// Start goroutine for each ticker
	for _, ticker := range tickers {
		wg.Add(1)
		go func(symbol string) {
			defer wg.Done()

			// Acquire semaphore
			sem <- struct{}{}
			defer func() { <-sem }()

			url := urls[symbol]

			// Fetch price using the global browser context
			price, err := pf.FetchPrice(ctx, url)

			// Send results
			results <- models.PriceResult{
				Symbol: symbol,
				Price:  price,
				Error:  err,
			}
		}(ticker)
	}

	go func() {
		wg.Wait()
		close(results)
	}()

	// Collect all results
	priceMap := make(map[string]models.PriceResult)
	for result := range results {
		priceMap[result.Symbol] = result
	}

	return priceMap, nil
}

// GetURLs creates a URL map for a list of tickers
func GetURLs(tickers []string) map[string]string {
	urls := make(map[string]string)
	for _, t := range tickers {
		urls[t] = fmt.Sprintf("https://finance.yahoo.com/quote/%s/", t)
	}
	return urls
}

// Cleanup should be called when the application is shutting down
func (pf *PriceFetcher) Cleanup() {
	cleanupGlobalBrowser()
}
