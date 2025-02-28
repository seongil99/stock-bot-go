package services

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strconv"
	"sync"
	"time"

	"stock-bot/models"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

// Database related error definitions
var (
	ErrMongoURINotSet      = errors.New("MONGODB_URI not set")
	ErrMongoConnection     = errors.New("failed to connect to MongoDB")
	ErrMongoQueryFailed    = errors.New("MongoDB query failed")
	ErrNoClosingPriceFound = errors.New("no closing price found for symbol")
	ErrInvalidPriceFormat  = errors.New("invalid price format")
)

// Database handles MongoDB connections and operations
type Database struct {
	client *mongo.Client
	config models.Config
}

// NewDatabase creates a new Database instance
func NewDatabase(mongoURI string) (*Database, error) {
	if mongoURI == "" {
		return nil, ErrMongoURINotSet
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	clientOptions := options.Client().ApplyURI(mongoURI)
	client, err := mongo.Connect(clientOptions)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrMongoConnection, err)
	}

	// Verify connection (using context)
	err = client.Ping(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrMongoConnection, err)
	}

	return &Database{
		client: client,
		config: models.DefaultConfig(),
	}, nil
}

// SavePrice saves stock price information to MongoDB
func (db *Database) SavePrice(symbol, price string, isClosing bool, wg *sync.WaitGroup) error {
	if wg != nil {
		defer wg.Done()
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	collection := db.client.Database("stock_data").Collection("stocks")
	stockData := models.MongoDTO{
		Symbol:    symbol,
		Price:     price,
		Timestamp: time.Now(),
		IsClosing: isClosing,
	}

	_, err := collection.InsertOne(ctx, stockData)
	if err != nil {
		log.Printf("Failed to insert stock data: %v", err)
		return fmt.Errorf("%w: %v", ErrMongoQueryFailed, err)
	}

	log.Printf("Saved %s: %s to MongoDB (closing: %v)", symbol, price, isClosing)
	return nil
}

// GetLatestClosingPrice retrieves the latest closing price for a specific stock
func (db *Database) GetLatestClosingPrice(symbol string) (float64, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	collection := db.client.Database("stock_data").Collection("stocks")

	filter := bson.D{{Key: "symbol", Value: symbol}, {Key: "isClosing", Value: true}}
	opts := options.FindOne().SetSort(bson.D{{Key: "timestamp", Value: -1}})

	var result models.MongoDTO
	err := collection.FindOne(ctx, filter, opts).Decode(&result)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return 0, fmt.Errorf("%w: %s", ErrNoClosingPriceFound, symbol)
		}
		return 0, fmt.Errorf("%w: %v", ErrMongoQueryFailed, err)
	}

	price, err := strconv.ParseFloat(result.Price, 64)
	if err != nil {
		return 0, fmt.Errorf("%w: %v", ErrInvalidPriceFormat, err)
	}

	return price, nil
}

// GetPriceHistory retrieves price history for a specific stock
func (db *Database) GetPriceHistory(symbol string, days int) ([]models.MongoDTO, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	collection := db.client.Database("stock_data").Collection("stocks")

	// Query for data from the specified number of previous days
	startDate := time.Now().AddDate(0, 0, -days)
	filter := bson.D{
		{Key: "symbol", Value: symbol},
		{Key: "timestamp", Value: bson.D{{Key: "$gte", Value: startDate}}},
		{Key: "isClosing", Value: true},
	}
	opts := options.Find().SetSort(bson.D{{Key: "timestamp", Value: 1}})

	cursor, err := collection.Find(ctx, filter, opts)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrMongoQueryFailed, err)
	}
	defer cursor.Close(ctx)

	var results []models.MongoDTO
	if err := cursor.All(ctx, &results); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrMongoQueryFailed, err)
	}

	return results, nil
}

// Close terminates the database connection
func (db *Database) Close() error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	return db.client.Disconnect(ctx)
}
