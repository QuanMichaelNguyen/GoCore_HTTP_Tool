package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/joho/godotenv"
	"github.com/redis/go-redis/v9"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

var (
	client      *mongo.Client
	postCol     *mongo.Collection
	redisClient *redis.Client
	ctx         = context.Background()
)

const (
	postCachePrefix = "post:"
	allPostsKey     = "all_posts"
	cacheDuration   = 10 * time.Minute
)

func initMongoDB() {
	var err error
	db_error := godotenv.Load()
	if db_error != nil {
		log.Fatal("Error loading .env file")
	}
	mongoURL := os.Getenv("MONGODB_URL")
	if mongoURL == "" {
		log.Fatal("MONGODB_URL is not set")
	}
	clientOptions := options.Client().ApplyURI(mongoURL).SetMaxPoolSize(100).SetMinPoolSize(5).SetMaxConnIdleTime(30 * time.Second)
	client, err = mongo.Connect(ctx, clientOptions)
	if err != nil {
		log.Fatal("MongoDB Connection Error:", err)
	}
	err = client.Ping(ctx, nil)
	if err != nil {
		log.Fatal("MongoDB Ping Error:", err)
	}
	// Create index on ID field for faster Lookups
	indexModel := mongo.IndexModel{
		Keys:    bson.M{"id": 1},
		Options: options.Index().SetUnique(true),
	}
	_, err = client.Database("Go").Collection("posts").Indexes().CreateOne(ctx, indexModel)
	postCol = client.Database("Go").Collection("posts")
	fmt.Println("Connected to MongoDB!")
}

func initRedis() {
	redisURL := os.Getenv("REDIS_URL")
	if redisURL == "" {
		redisURL = "localhost:6379"
	}
	redisPassword := os.Getenv("REDIS_PASSWORD")
	redisDB, _ := strconv.Atoi(os.Getenv("REDIS_DB"))

	redisClient = redis.NewClient(&redis.Options{
		Addr:         redisURL,
		Password:     redisPassword,
		DB:           redisDB,
		PoolSize:     50,
		MinIdleConns: 10,
	})

	_, err := redisClient.Ping(ctx).Result()
	if err != nil {
		log.Printf("Warning: Redis connection failed: %v", err)
		log.Println("Continuing without Redis cache")
		redisClient = nil
	} else {
		fmt.Println("Connected to Redis!")
	}
}

// define c Post class with ID, Body attributes
type Post struct {
	ID   int    `json:"id"`
	Body string `json:"body"`
}

var (
	nextID  = 1        // variable helps us to make unique post ids when making new post
	postsMu sync.Mutex // mutex to lock programwhen changing to the posts map (concurrent request causes race condition --> access the same resources at the same time)

)

// Initialize nextID from the database
func initNextID() {
	var result struct {
		MaxID int `bson:"maxID"`
	}
	pipeline := mongo.Pipeline{
		{{"$sort", bson.D{{"id", -1}}}},
		{{"$limit", 1}},
		{{"$project", bson.D{{"maxID", "$id"}}}},
	}
	cursor, err := postCol.Aggregate(ctx, pipeline)
	if err != nil {
		log.Printf("Failed to get max ID: %v", err)
		return
	}
	defer cursor.Close(ctx)

	if cursor.Next(ctx) {
		if err := cursor.Decode(&result); err == nil {
			nextID = result.MaxID + 1
			log.Printf("Next ID set to: %d", nextID)
		}
	}
}

func cachePost(post Post) {
	if redisClient == nil {
		return
	}
	key := fmt.Sprintf("%s%d", postCachePrefix, post.ID)
	data, err := json.Marshal(post)
	if err != nil {
		log.Printf("Error marshaling post for cache: %v", err)
		return
	}
	err = redisClient.Set(ctx, key, data, cacheDuration).Err()
	if err != nil {
		log.Printf("Error caching post: %v", err)
	}
}
func getCachedPost(id int) (Post, bool) {
	var post Post

	if redisClient == nil {
		return post, false
	}

	key := fmt.Sprintf("%s%d", postCachePrefix, id)
	data, err := redisClient.Get(ctx, key).Bytes()
	if err != nil {
		return post, false
	}

	err = json.Unmarshal(data, &post)
	if err != nil {
		log.Printf("Error unmarshaling cached post: %v", err)
		return post, false
	}

	return post, true
}
func invalidatePostCache(id int) {
	if redisClient == nil {
		return
	}

	key := fmt.Sprintf("%s%d", postCachePrefix, id)
	err := redisClient.Del(ctx, key).Err()
	if err != nil {
		log.Printf("Error invalidating post cache: %v", err)
	}

	// Also invalidate all posts cache
	redisClient.Del(ctx, allPostsKey)
}

func cacheAllPosts(posts []Post) {
	if redisClient == nil {
		return
	}

	data, err := json.Marshal(posts)
	if err != nil {
		log.Printf("Error marshaling all posts for cache: %v", err)
		return
	}

	err = redisClient.Set(ctx, allPostsKey, data, cacheDuration).Err()
	if err != nil {
		log.Printf("Error caching all posts: %v", err)
	}
}

func getCachedAllPosts() ([]Post, bool) {
	var posts []Post

	if redisClient == nil {
		return posts, false
	}

	data, err := redisClient.Get(ctx, allPostsKey).Bytes()
	if err != nil {
		return posts, false
	}

	err = json.Unmarshal(data, &posts)
	if err != nil {
		log.Printf("Error unmarshaling cached posts: %v", err)
		return posts, false
	}

	return posts, true
}

// Implementing server
// Entry point for module
func main() {
	// setup handlers for the /posts and /posts routes
	initMongoDB()
	initRedis()
	initNextID()
	http.HandleFunc("/posts", postsHandler)
	http.HandleFunc("/posts/", postHandler)
	// http.HandleFunc("/edit/", editHandler)

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
	log.Fatal((http.ListenAndServe(":8080", nil)))
}

// Handling function for /posts endpoint
func postsHandler(w http.ResponseWriter, r *http.Request) { // (return JSON, information about the incoming request)
	// check the HTTP requests methods
	switch r.Method {
	// if it's GET --> call the function to handle get request
	case "GET":
		handleGetPosts(w, r)
	case "POST":
		handlePostPosts(w, r)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func postHandler(w http.ResponseWriter, r *http.Request) { // (return JSON, information about the incoming request)
	// Debug printing
	idStr := r.URL.Path[len("/posts/"):]
	fmt.Printf("Path: %s\n", r.URL.Path)
	fmt.Printf("Extracted ID string: %s\n", idStr)
	//
	id, err := strconv.Atoi(idStr)
	if err != nil {
		http.Error(w, "Invalid post ID", http.StatusBadRequest)
		return
	}
	switch r.Method {
	case "GET":
		handleGetPost(w, r, id)
	case "DELETE":
		handleDeletePost(w, r, id)
	case "PUT":
		handleEditPost(w, r, id)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func handleGetPosts(w http.ResponseWriter, r *http.Request) {
	/*
		Using mutex to lock the server --> manipulate the posts map without
		worrying about another request trying to do the same thing at the same time
	*/
	// postsMu.Lock()

	// defer postsMu.Unlock() // defer until the code finished executing

	// Try to get from cache first
	if cachedPosts, found := getCachedAllPosts(); found {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(cachedPosts)
		return
	}

	// Parse pagination parameters
	limitStr := r.URL.Query().Get("limit")
	offsetStr := r.URL.Query().Get("offset")

	limit := 10 // default limit
	offset := 0 // default offset

	if limitStr != "" {
		if parsedLimit, err := strconv.Atoi(limitStr); err == nil && parsedLimit > 0 {
			limit = parsedLimit
		}
	}

	if offsetStr != "" {
		if parsedOffset, err := strconv.Atoi(offsetStr); err == nil && parsedOffset >= 0 {
			offset = parsedOffset
		}
	}

	// Set up options for pagination
	findOptions := options.Find().
		SetLimit(int64(limit)).
		SetSkip(int64(offset)).
		SetSort(bson.D{{"id", 1}})

	cursor, err := postCol.Find(ctx, bson.M{}, findOptions)
	if err != nil {
		http.Error(w, "Error fetching posts", http.StatusInternalServerError)
		return
	}
	defer cursor.Close(ctx)

	// Copying the posts to a new slice of type
	// ps := make([]Post, 0, len(posts))

	/*
		When you execute a query like postCol.Find(ctx, bson.M{}), it returns a cursor.
		This cursor points to the first document in the query result and can be used to \
		traverse the subsequent documents.

		cursor === pointer

	*/

	var ps []Post
	if err := cursor.All(ctx, &ps); err != nil {
		http.Error(w, "Error decoding posts", http.StatusInternalServerError)
		return
	}

	// Cache the results
	cacheAllPosts(ps)

	count, err := postCol.CountDocuments(ctx, bson.M{})
	if err != nil {
		log.Printf("Error counting documents: %v", err)
	}
	// Create response with pagination metadata
	type Response struct {
		Posts      []Post `json:"posts"`
		TotalPosts int64  `json:"totalPosts"`
		Limit      int    `json:"limit"`
		Offset     int    `json:"offset"`
	}
	response := Response{
		Posts:      ps,
		TotalPosts: count,
		Limit:      limit,
		Offset:     offset,
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// Loop through posts map and append post Struct to ps
// for _, p := range posts {
// 	ps = append(ps, p)
// }
// 	w.Header().Set("Content-Type", "application/json")
// 	json.NewEncoder(w).Encode(ps)

// }

func handlePostPosts(w http.ResponseWriter, r *http.Request) {
	var p Post

	// Read the entire body into a byte slice
	body, err := io.ReadAll(r.Body)

	if err != nil {
		http.Error(w, "Error reading request body", http.StatusInternalServerError)
		return
	}

	// Parse the body === JSON.parse
	if err := json.Unmarshal(body, &p); err != nil {
		http.Error(w, "Error parsing request body", http.StatusBadRequest)
		return
	}

	postsMu.Lock()
	defer postsMu.Unlock()

	p.ID = nextID
	nextID++

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// posts[p.ID] = p
	_, err = postCol.InsertOne(ctx, p)
	if err != nil {
		http.Error(w, "Error creating post", http.StatusInternalServerError)
		return
	}

	// Invalidate all posts cache since we've added a new one
	redisClient.Del(ctx, allPostsKey)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(p)
}

func handleGetPost(w http.ResponseWriter, r *http.Request, id int) {
	startTime := time.Now()
	var source string
	// Try to get from cache first
	if post, found := getCachedPost(id); found {
		source = "cache"
		elapsed := time.Since(startTime).Milliseconds()

		// Create response with metadata
		type ResponseWithMeta struct {
			Post           Post   `json:"post"`
			Source         string `json:"source"`
			ResponseTimeMs int64  `json:"responseTimeMs"`
		}

		respWithMeta := ResponseWithMeta{
			Post:           post,
			Source:         source,
			ResponseTimeMs: elapsed,
		}

		log.Printf("POST %d SERVED FROM CACHE in %dms", id, elapsed)

		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("X-Cache", "HIT")
		w.Header().Set("X-Response-Time-Ms", fmt.Sprintf("%d", elapsed))
		json.NewEncoder(w).Encode(respWithMeta)
		return
	}

	var p Post
	source = "database"
	/*
		Decode method takes the result of the FindOne query and
		decodes the retrieved document into a Go variable
	*/
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := postCol.FindOne(ctx, bson.M{"id": id}).Decode(&p)
	if err != nil {
		http.Error(w, "Id not found", http.StatusNotFound)
		return
	}
	cachePost(p)
	elapsed := time.Since(startTime).Milliseconds()

	// Create response with metadata
	type ResponseWithMeta struct {
		Post           Post   `json:"post"`
		Source         string `json:"source"`
		ResponseTimeMs int64  `json:"responseTimeMs"`
	}

	respWithMeta := ResponseWithMeta{
		Post:           p,
		Source:         source,
		ResponseTimeMs: elapsed,
	}

	log.Printf("POST %d SERVED FROM DATABASE in %dms", id, elapsed)

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("X-Cache", "MISS")
	w.Header().Set("X-Response-Time-Ms", fmt.Sprintf("%d", elapsed))
	json.NewEncoder(w).Encode(respWithMeta)
}

func handleDeletePost(w http.ResponseWriter, r *http.Request, id int) {
	postsMu.Lock()
	defer postsMu.Unlock()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// If you use a two-value assignment for accessing a
	// value on a map, you get the value first then an
	// "exists" variable.
	res, err := postCol.DeleteOne(ctx, bson.M{"id": id})
	if err != nil {
		http.Error(w, "Post not found", http.StatusNotFound)
		return
	}
	// delete(posts, id)
	if res.DeletedCount == 0 {
		http.Error(w, "No ID to be deleted", http.StatusNotFound)
		return
	}
	// Invalidate cache
	invalidatePostCache(id)

	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"message": "Post deleted successfully"}`))
}

func handleEditPost(w http.ResponseWriter, r *http.Request, id int) { // (return JSON, information about the incoming request)

	// Decode the request body into a map[string]interface{}
	var updates map[string]interface{}                               // updates = {string key: any types of values}
	if err := json.NewDecoder(r.Body).Decode(&updates); err != nil { // reads and decodes the JSON body of the request (r.Body) into the updates map
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}
	// checks if the updates map contains a key "Body". If it does, it assigns the value associated with this key to newBody
	// if newBody, ok := updates["Body"]; ok {
	// 	if bodyStr, ok := newBody.(string); ok {
	// 		p.Body = bodyStr
	// 	} else {
	// 		http.Error(w, "We need string type", http.StatusBadRequest)
	// 		return
	// 	}
	// }

	// posts[id] = p
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	update := bson.M{"$set": updates}
	res, err := postCol.UpdateOne(ctx, bson.M{"id": id}, update)
	if err != nil {
		http.Error(w, "Error updating post", http.StatusInternalServerError)
		return
	}
	if res.MatchedCount == 0 {
		http.Error(w, "Post not found", http.StatusNotFound)
		return
	}
	invalidatePostCache(id)
	// Get the updated post
	var updatedPost Post
	err = postCol.FindOne(ctx, bson.M{"id": id}).Decode(&updatedPost)
	if err != nil {
		http.Error(w, "Error retrieving updated post", http.StatusInternalServerError)
		return
	}
	// Cache the updated post
	cachePost(updatedPost)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(updatedPost)
}
