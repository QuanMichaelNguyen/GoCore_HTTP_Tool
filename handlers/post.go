package handlers

import (
	"context"
	"encoding/json"
	"go-server/cache"
	"go-server/db"
	"go-server/models"
	"go-server/utils"
	"io"
	"log"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/go-redis/redis"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo/options"
)

var (
	redisClient *redis.Client
	nextID      = 1        // variable helps us to make unique post ids when making new post
	postsMu     sync.Mutex // mutex to lock programwhen changing to the posts map (concurrent request causes race condition --> access the same resources at the same time)
)

type PaginatedResponse struct {
	Posts      []models.Post `json:"posts"`
	TotalPosts int64         `json:"totalPosts"`
	Limit      int           `json:"limit"`
	Offset     int           `json:"offset"`
}

const (
	postCachePrefix = "post:"
	allPostsKey     = "all_posts"
	cacheDuration   = 10 * time.Minute
)

// Handling function for /posts endpoint
func PostsHandler(w http.ResponseWriter, r *http.Request) { // (return JSON, information about the incoming request)
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

func PostHandler(w http.ResponseWriter, r *http.Request) { // (return JSON, information about the incoming request)
	// Debug printing
	idStr := r.URL.Path[len("/posts/"):]
	id, err := strconv.Atoi(idStr)
	if err != nil {
		http.Error(w, "Invalid post ID", http.StatusBadRequest)
		return
	}
	switch r.Method {
	case http.MethodGet:
		handleGetPost(w, r, id)
	case http.MethodDelete:
		handleDeletePost(w, r, id)
	case http.MethodPut:
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
	if cachedPosts, found := cache.GetCachedAllPosts(); found {
		utils.RespondWithJSON(w, cachedPosts)
		return
	}

	limit, offset := utils.ParsePaginationParams(r)
	findOptions := options.Find().SetLimit(int64(limit)).SetSkip(int64(offset)).SetSort(bson.D{{"id", 1}})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	cursor, err := db.PostCol.Find(ctx, bson.M{}, findOptions)
	if err != nil {
		http.Error(w, "Error fetching posts", http.StatusInternalServerError)
		return
	}
	defer cursor.Close(ctx)

	var ps []models.Post
	if err := cursor.All(ctx, &ps); err != nil {
		http.Error(w, "Error decoding posts", http.StatusInternalServerError)
		return
	}

	cache.CacheAllPosts(ps)

	count, _ := db.PostCol.CountDocuments(ctx, bson.M{})
	utils.RespondWithJSON(w, PaginatedResponse{Posts: ps, TotalPosts: count, Limit: limit, Offset: offset})
}

func handlePostPosts(w http.ResponseWriter, r *http.Request) {
	var p models.Post
	body, err := io.ReadAll(r.Body)
	if err != nil {
		log.Printf("Error reading request body: %v", err)
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}
	
	if err := json.Unmarshal(body, &p); err != nil {
		log.Printf("Error unmarshaling JSON: %v", err)
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	postsMu.Lock()
	defer postsMu.Unlock()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Get the next available ID from the database
	var maxIDResult struct {
		MaxID int `bson:"maxID"`
	}
	pipeline := []bson.M{
		{"$sort": bson.M{"id": -1}},
		{"$limit": 1},
		{"$project": bson.M{"maxID": "$id"}},
	}

	cursor, err := db.PostCol.Aggregate(ctx, pipeline)
	if err != nil {
		log.Printf("Error getting max ID: %v", err)
		http.Error(w, "Error creating post", http.StatusInternalServerError)
		return
	}
	defer cursor.Close(ctx)

	if cursor.Next(ctx) {
		if err := cursor.Decode(&maxIDResult); err != nil {
			log.Printf("Error decoding max ID: %v", err)
			http.Error(w, "Error creating post", http.StatusInternalServerError)
			return
		}
		p.ID = maxIDResult.MaxID + 1
	} else {
		p.ID = 1 // If no posts exist, start with ID 1
	}

	if db.PostCol == nil {
		log.Printf("MongoDB collection is nil")
		http.Error(w, "Database connection error", http.StatusInternalServerError)
		return
	}

	insertResult, err := db.PostCol.InsertOne(ctx, p)
	if err != nil {
		log.Printf("Error inserting post: %v", err)
		http.Error(w, "Error creating post", http.StatusInternalServerError)
		return
	}

	log.Printf("Successfully inserted post with ID: %v", insertResult.InsertedID)
	cache.InvalidatePostCache(p.ID)
	utils.RespondWithStatus(w, http.StatusCreated, p)
}

func handleGetPost(w http.ResponseWriter, r *http.Request, id int) {
	start := time.Now()
	if post, found := cache.GetCachedPost(id); found {
		cachedPost := models.Post{
			ID:   post.ID,
			Body: post.Body,
		}

		utils.RespondWithMetadata(w, cachedPost, "cache", time.Since(start).Milliseconds(), true)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var p models.Post
	if err := db.PostCol.FindOne(ctx, bson.M{"id": id}).Decode(&p); err != nil {
		http.Error(w, "Post not found", http.StatusNotFound)
		return
	}
	cachePost := cache.Post{
		ID:   p.ID,
		Body: p.Body,
	}
	cache.CachePost(cachePost)
	utils.RespondWithMetadata(w, p, "database", time.Since(start).Milliseconds(), false)
}

func handleDeletePost(w http.ResponseWriter, r *http.Request, id int) {
	postsMu.Lock()
	defer postsMu.Unlock()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	res, err := db.PostCol.DeleteOne(ctx, bson.M{"id": id})
	if err != nil || res.DeletedCount == 0 {
		http.Error(w, "Post not found", http.StatusNotFound)
		return
	}

	cache.InvalidatePostCache(id)
	w.Write([]byte(`{"message": "Post deleted successfully"}`))
}

func handleEditPost(w http.ResponseWriter, r *http.Request, id int) { // (return JSON, information about the incoming request)

	var updates map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&updates); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	update := bson.M{"$set": updates}
	res, err := db.PostCol.UpdateOne(ctx, bson.M{"id": id}, update)
	if err != nil || res.MatchedCount == 0 {
		http.Error(w, "Post not found", http.StatusNotFound)
		return
	}

	cache.InvalidatePostCache(id)

	var updatedPost models.Post
	if err := db.PostCol.FindOne(ctx, bson.M{"id": id}).Decode(&updatedPost); err != nil {
		http.Error(w, "Error retrieving updated post", http.StatusInternalServerError)
		return
	}
	utils.RespondWithJSON(w, updatedPost)
}
