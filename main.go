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

	"github.com/joho/godotenv"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

var (
	client  *mongo.Client
	postCol *mongo.Collection
	ctx     = context.Background()
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
	clientOptions := options.Client().ApplyURI(mongoURL)
	client, err = mongo.Connect(ctx, clientOptions)
	if err != nil {
		log.Fatal("MongoDB Connection Error:", err)
	}
	err = client.Ping(ctx, nil)
	if err != nil {
		log.Fatal("MongoDB Ping Error:", err)
	}
	postCol = client.Database("Go").Collection("posts")
	fmt.Println("Connected to MongoDB!")
}

// define c Post class with ID, Body attributes
type Post struct {
	ID   int    `json:"id"`
	Body string `json:"body"`
}

var (
	posts   = make(map[int]Post) // hold our posts in memory
	nextID  = 1                  // variable helps us to make unique post ids when making new post
	postsMu sync.Mutex           // mutex to lock programwhen changing to the posts map (concurrent request causes race condition --> access the same resources at the same time)

)

// Implementing server
// Entry point for module
func main() {
	// setup handlers for the /posts and /posts routes
	initMongoDB()
	http.HandleFunc("/posts", postsHandler)
	http.HandleFunc("/posts/", postHandler)
	// http.HandleFunc("/edit/", editHandler)

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
	id, err := strconv.Atoi(r.URL.Path[len("/posts/"):])
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
	postsMu.Lock()

	defer postsMu.Unlock() // defer until the code finished executing

	cursor, err := postCol.Find(ctx, bson.M{})
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

	// Loop through posts map and append post Struct to ps
	// for _, p := range posts {
	// 	ps = append(ps, p)
	// }
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(ps)
}

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
	// posts[p.ID] = p
	postCol.InsertOne(ctx, p)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(p)
}

func handleGetPost(w http.ResponseWriter, r *http.Request, id int) {
	postsMu.Lock()
	defer postsMu.Unlock()

	var p Post
	/*
		Decode method takes the result of the FindOne query and
		decodes the retrieved document into a Go variable
	*/
	err := postCol.FindOne(ctx, bson.M{"id": id}).Decode(&p)
	if err != nil {
		http.Error(w, "Id not found", http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(p)
}

func handleDeletePost(w http.ResponseWriter, r *http.Request, id int) {
	postsMu.Lock()
	defer postsMu.Unlock()

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
	w.WriteHeader(http.StatusOK)
}

func handleEditPost(w http.ResponseWriter, r *http.Request, id int) { // (return JSON, information about the incoming request)
	postsMu.Lock()
	defer postsMu.Unlock()

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

	handleGetPost(w, r, id)
	w.WriteHeader(http.StatusOK)
}
