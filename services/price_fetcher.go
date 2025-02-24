package services

import (
	"context"
	"fmt"
	"log"

	"github.com/chromedp/chromedp"
)

type PriceFetcher struct {
	Opts []chromedp.ExecAllocatorOption
}

func NewPriceFetcher() *PriceFetcher {
	opts := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.DisableGPU,
		chromedp.NoDefaultBrowserCheck,
		chromedp.NoFirstRun,
		chromedp.Headless,
		chromedp.NoSandbox,
		chromedp.Flag("blink-settings", "scriptEnabled=false, imagesEnabled=false"),
		chromedp.Flag("disable-extensions", true),
	)
	return &PriceFetcher{Opts: opts}
}

func (pf *PriceFetcher) FetchPrice(ctx context.Context, url string) (string, error) {
	var price string
	log.Printf("Fetching price from %s", url)
	err := chromedp.Run(ctx,
		chromedp.Navigate(url),
		chromedp.WaitVisible(`span[data-testid="qsp-price"]`, chromedp.ByQuery),
		chromedp.Text(`span[data-testid="qsp-price"]`, &price, chromedp.ByQuery),
	)
	return price, err
}

func GetURLs(tickers []string) map[string]string {
	urls := make(map[string]string)
	for _, t := range tickers {
		urls[t] = fmt.Sprintf("https://finance.yahoo.com/quote/%s/", t)
	}
	return urls
}
