package db

import (
	"context"
	"crypto/tls"
	"fmt"
	"log"
	"os"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

var (
	Client  *mongo.Client
	PostCol *mongo.Collection
	ctx     = context.Background()
)

func InitMongoDB() {
	mongoURL := os.Getenv("MONGODB_URL")
	if mongoURL == "" {
		log.Fatal("MONGODB_URL is not set")
	}
	// Create context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	clientOptions := buildMongoClientOptions(mongoURL)
	var err error

	Client, err = mongo.Connect(ctx, clientOptions)
	if err != nil {
		log.Fatal("MongoDB Connection Error:", err)
	}
	if err = Client.Ping(ctx, nil); err != nil {
		log.Fatal("MongoDB Ping Error: %v", err)
	}

	PostCol = Client.Database("Go").Collection("posts")
	if err := ensurePostIndex(ctx, PostCol); err != nil {
		log.Fatalf("Failed to create index: %v", err)
	}
	fmt.Println("Connected to MongoDB!")
}

func buildMongoClientOptions(uri string) *options.ClientOptions {
	// Configure TLS properly
	tlsConfig := &tls.Config{
		MinVersion: tls.VersionTLS12,
	}

	return options.Client().
		ApplyURI(uri).
		SetMaxPoolSize(100).
		SetMinPoolSize(5).
		SetMaxConnIdleTime(30 * time.Second).
		SetTLSConfig(tlsConfig)
}

func ensurePostIndex(ctx context.Context, col *mongo.Collection) error {
	indexModel := mongo.IndexModel{
		Keys:    bson.M{"id": 1},
		Options: options.Index().SetUnique(true),
	}
	_, err := col.Indexes().CreateOne(ctx, indexModel)
	return err
}
