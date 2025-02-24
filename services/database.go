package services

import (
	"context"
	"fmt"
	"log"
	"os"
	"sync"
	"time"

	"stock-bot/models"

	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

type Database struct {
	client *mongo.Client
}

func NewDatabase() (*Database, error) {
	mongoURI := os.Getenv("MONGODB_URI")
	if mongoURI == "" {
		return nil, fmt.Errorf("MONGODB_URI not set")
	}

	client, err := mongo.Connect(options.Client().ApplyURI(mongoURI))
	if err != nil {
		return nil, err
	}

	return &Database{client: client}, nil
}

func (db *Database) SavePrice(symbol, price string, wg *sync.WaitGroup) {
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
	var result models.MongoDTO
	err = collection.FindOne(context.Background(), map[string]string{"symbol": symbol}).Decode(&result)
	if err != nil {
		log.Fatal("Failed to find stock data: ", err)
	}
	log.Printf("Found %s: %s in MongoDB", result.Symbol, result.Price)

	log.Printf("Saved %s: %s to MongoDB", symbol, price)
}
