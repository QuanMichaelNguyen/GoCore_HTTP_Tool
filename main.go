package main

import (
	"context"
	"fmt"
	"go-server/cache"
	"go-server/db"
	"go-server/handlers"
	"log"
	"net/http"
	"os"
	"sync"

	"github.com/go-redis/redis"
	"github.com/joho/godotenv"
	"github.com/rs/cors"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
)

// define c Post class with ID, Body attributes

var (
	nextID      = 1        // variable helps us to make unique post ids when making new post
	postsMu     sync.Mutex // mutex to lock programwhen changing to the posts map (concurrent request causes race condition --> access the same resources at the same time)
	ctx         = context.Background()
	client      *mongo.Client
	redisClient *redis.Client
)

func initNextID() {
	// Ensure MongoDB client and collection are initialized
	if db.PostCol == nil {
		log.Fatal("MongoDB collection is nil. Cannot initialize nextID.")
	}

	var result struct {
		MaxID int `bson:"maxID"`
	}
	// MongoDB aggregation pipeline to get the max ID
	pipeline := mongo.Pipeline{
		{{"$sort", bson.D{{"id", -1}}}},
		{{"$limit", 1}},
		{{"$project", bson.D{{"maxID", "$id"}}}},
	}

	cursor, err := db.PostCol.Aggregate(context.Background(), pipeline)
	if err != nil {
		log.Printf("Failed to aggregate max ID: %v", err)
		return
	}
	defer cursor.Close(context.Background())

	if cursor.Next(context.Background()) {
		if err := cursor.Decode(&result); err == nil {
			nextID = result.MaxID + 1
			log.Printf("Next ID set to: %d", nextID)
		}
	}
}

// Implementing server
// Entry point for module
func main() {
	if os.Getenv("ENV") != "production" {
		if err := godotenv.Load(); err != nil {
			log.Println("No .env file found, continuing...")
		}
	}
	db.InitMongoDB()
	fmt.Println("MongoDB Collection initialized:", db.PostCol)
	cache.InitRedis()
	initNextID()

	// Create a new mux router
	mux := http.NewServeMux()

	// setup handlers for the /posts and /posts routes
	mux.HandleFunc("/posts", handlers.PostsHandler)
	mux.HandleFunc("/posts/", handlers.PostHandler)

	// Configure CORS
	c := cors.New(cors.Options{
		AllowedOrigins:   []string{"http://localhost:3000", "http://localhost:3001"},
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Content-Type", "Authorization"},
		AllowCredentials: true,
	})

	// Wrap the mux with CORS middleware
	handler := c.Handler(mux)

	// Graceful shutdown handling
	ctx, cancel := context.WithCancel(context.Background())
	fmt.Println(ctx)
	defer cancel()

	go func() {
		c := make(chan os.Signal, 1)
		// signal.Notify(c, os.Interrupt, syscall.SIGTERM)
		<-c
		log.Println("Shutting down...")
		cancel()

		// Close MongoDB connection
		if err := client.Disconnect(context.Background()); err != nil {
			log.Printf("MongoDB disconnect error: %v", err)
		}

		// Close Redis connection
		if redisClient != nil {
			if err := redisClient.Close(); err != nil {
				log.Printf("Redis close error: %v", err)
			}
		}

		os.Exit(0)
	}()

	fmt.Println("Server is running at http://localhost:8080")
	/*
		log: recording program events, including errors
		log.Fatal(): logs a message and then calls os.Exit(1), terminating the program
		http.ListenAndServe: starts an HTTP server, port 8080
		nil: use default HTTP handler

		==> start an HTTP server

	*/
	log.Fatal(http.ListenAndServe(":8080", handler))
}
