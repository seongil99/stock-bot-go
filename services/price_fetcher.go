package services

import (
	"context"
	"errors"
	"fmt"
	"log"
	"sync"
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

// PriceFetcher collects stock price information
type PriceFetcher struct {
	Opts          []chromedp.ExecAllocatorOption
	FetchTimeout  time.Duration
	MaxRetries    int
	RetryInterval time.Duration
}

// NewPriceFetcher creates a new PriceFetcher instance
func NewPriceFetcher() *PriceFetcher {
	opts := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.DisableGPU,
		chromedp.NoDefaultBrowserCheck,
		chromedp.NoFirstRun,
		chromedp.Headless,
		chromedp.NoSandbox,
		chromedp.Flag("blink-settings", "scriptEnabled=false, imagesEnabled=false"),
		chromedp.Flag("disable-extensions", true),
		chromedp.Flag("disable-dev-shm-usage", true), // Improves stability in container environments
		chromedp.Flag("disable-setuid-sandbox", true),
		chromedp.Flag("no-zygote", true),
		chromedp.UserAgent("AppleWebKit/537.36 (KHTML, like Gecko) Chrome/64.0.3282.140 Safari/537.36"),
		chromedp.Flag("ignore-certificate-errors", true),
		chromedp.Flag("disable-features", "IsolateOrigins,site-per-process"),
		chromedp.Flag("disable-web-security", true),
		chromedp.Flag("single-process", true),
		chromedp.Flag("no-default-browser-check", true),
	)
	return &PriceFetcher{
		Opts:          opts,
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

		// Use timeout context
		timeoutCtx, cancel := context.WithTimeout(ctx, pf.FetchTimeout)

		err = func() error {
			defer cancel()
			return chromedp.Run(timeoutCtx,
				chromedp.Navigate(url),
				chromedp.WaitVisible(`span[data-testid="qsp-price"]`, chromedp.ByQuery),
				chromedp.Text(`span[data-testid="qsp-price"]`, &price, chromedp.ByQuery),
			)
		}()

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
	// Setup base context
	allocCtx, allocCancel := chromedp.NewExecAllocator(ctx, pf.Opts...)
	defer allocCancel()

	// Shared parent context
	parentCtx, parentCancel := chromedp.NewContext(allocCtx, chromedp.WithLogf(log.Printf))
	defer parentCancel()

	// Semaphore to limit concurrency
	sem := make(chan struct{}, maxConcurrency)

	// Results and error channels
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

			// Create child context
			childCtx, childCancel := chromedp.NewContext(parentCtx)
			defer childCancel()

			// Fetch price
			price, err := pf.FetchPrice(childCtx, url)

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
