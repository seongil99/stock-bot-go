# Stock Price Bot

A real-time stock price monitoring and alerting system that tracks stock prices, sends daily reports, and notifies of significant price changes.

## Features

- **Daily Stock Reports**: Automatically sends a daily summary of stock prices at a configurable time (default: 7:00 AM)
- **Real-time Price Alerts**: Monitors stock prices during market hours and sends alerts for significant price changes (default: 5% threshold)
- **Once-per-day Alert Limiting**: Each stock will only trigger one alert per day, preventing alert fatigue
- **Multiple Messaging Platforms**: Supports both Telegram and Line for notifications
- **Persistent Storage**: Stores historical price data in MongoDB for trend analysis
- **Configurable Settings**: Customizable alert thresholds, check intervals, and reporting times

## Technology Stack

- **Go**: Core application written in Go
- **MongoDB**: Data storage for historical price information
- **ChromeDP**: Headless browser automation for fetching stock prices
- **Docker**: Containerized deployment for easy setup and scaling
- **Telegram/Line API**: Messaging integrations for notifications

## Getting Started

### Prerequisites

- Docker and Docker Compose
- MongoDB (or use the provided Docker Compose configuration)
- Telegram Bot Token and Chat ID (optional)
- Line Channel Access Token (optional)

### Environment Variables

Create a `.env` file with the following variables:

```
TELEGRAM_BOT_TOKEN=your_telegram_bot_token
TELEGRAM_CHAT_ID=your_telegram_chat_id
LINE_CHANNEL_ACCESS_TOKEN=your_line_channel_access_token

MONGO_INITDB_ROOT_USERNAME=username
MONGO_INITDB_ROOT_PASSWORD=password
MONGODB_URI=mongodb://username:password@mongo:27017/stock_data?authSource=admin


TIMEZONE=Asia/Seoul
CHECK_HOUR=7
```

### Installation and Setup

1. Clone the repository:
   ```
   git clone https://github.com/yourusername/stock-bot.git
   cd stock-bot
   ```

2. Build and start the containers:
   ```
   docker-compose up -d
   ```

3. Check the logs to verify the application is running:
   ```
   docker-compose logs -f stock-bot
   ```

## Configuration

### Stock List

Edit the `models/types.go` file to modify the list of stocks to monitor:

```go
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
```

### Alert Settings

Modify the constants in `main.go` to adjust alert behavior:

```go
const (
	alertThreshold       = 5.0  // Alert threshold for price changes over 5%
	maxConcurrency       = 5    // Maximum number of concurrent requests
	checkInterval        = 15   // Scheduler check interval in minutes
	defaultCheckHour     = 7    // Default time for daily report (7AM)
	realtimeCheckMinutes = 30   // Interval for realtime price checks in minutes
)
```

## Docker Deployment

The project includes a `docker-compose.yml` file for easy deployment:

## Project Structure

```
stock-bot/
├── main.go                  # Main application entry point
├── models/
│   └── types.go             # Data models and structures
├── services/
│   ├── database.go          # MongoDB interactions
│   ├── messenger.go         # Messaging service interfaces
│   └── price_fetcher.go     # Stock price fetching logic
├── Dockerfile               # Container definition
├── docker-compose.yml       # Multi-container setup
└── README.md                # Project documentation
```

## How It Works

1. **Initialization**: The application loads configuration from environment variables and connects to MongoDB.
2. **Scheduler**: A ticker runs every 15 minutes to check if actions need to be taken.
3. **Daily Report**: At the configured time (default: 7 AM), the system fetches current prices for all stocks and sends a summary.
4. **Real-time Monitoring**: During market hours, the system checks prices every 30 minutes and compares them with previous closing prices.
5. **Alerts**: If a price change exceeds the threshold (default: 5%), an alert is sent (limited to once per day per stock).


