package cache

import (
	"context"
	"encoding/json"
	"fmt"
	"go-server/models"
	"log"
	"os"
	"strconv"
	"time"

	"github.com/go-redis/redis"
)

type Post struct {
	ID   int    `json:"id"`
	Body string `json:"body"`
}

var (
	redisClient *redis.Client
	ctx         = context.Background()
)

const (
	postCachePrefix = "post:"
	allPostsKey     = "all_posts"
	cacheDuration   = 10 * time.Minute
)

func InitRedis() {
	redisURL, redisPassword, redisDB := getRedisConfig()

	redisClient = redis.NewClient(&redis.Options{
		Addr:         redisURL,
		Password:     redisPassword,
		DB:           redisDB,
		PoolSize:     50,
		MinIdleConns: 10,
	})

	if err := testRedisConnection(); err != nil {
		log.Printf("Warning: Redis connection failed: %v", err)
		log.Println("Continuing without Redis cache")
		redisClient = nil
	} else {
		fmt.Println("Connected to Redis!")
	}
}

func getRedisConfig() (string, string, int) {
	redisURL := os.Getenv("REDIS_URL")
	if redisURL == "" {
		redisURL = "localhost:6379"
	}
	redisPassword := os.Getenv("REDIS_PASSWORD")
	redisDB, err := strconv.Atoi(os.Getenv("REDIS_DB"))

	if err != nil {
		redisDB = 0
	}
	return redisURL, redisPassword, redisDB
}

func testRedisConnection() error {
	_, err := redisClient.Ping().Result()
	return err
}

func CachePost(post Post) {
	if redisClient == nil {
		return
	}
	cacheKey := BuildPostKey(post.ID)
	StoreInCache(cacheKey, post)
}
func GetCachedPost(id int) (Post, bool) {
	if redisClient == nil {
		return Post{}, false
	}

	var post Post
	cacheKey := BuildPostKey(id)

	if found := FetchFromCache(cacheKey, &post); !found {
		return Post{}, false
	}

	return post, true
}
func InvalidatePostCache(id int) {
	if redisClient == nil {
		return
	}

	keys := []string{BuildPostKey(id), allPostsKey}
	if err := redisClient.Del(keys...).Err(); err != nil {
		log.Printf("Error invalidating cache: %v", err)
	}
}

func CacheAllPosts(posts []models.Post) {
	if redisClient == nil {
		return
	}
	StoreInCache(allPostsKey, posts)
}

func GetCachedAllPosts() ([]Post, bool) {
	if redisClient == nil {
		return nil, false
	}

	var posts []Post
	if found := FetchFromCache(allPostsKey, &posts); !found {
		return nil, false
	}

	return posts, true
}

func BuildPostKey(id int) string {
	return fmt.Sprintf("%s%d", postCachePrefix, id)
}

func StoreInCache(key string, value interface{}) {
	data, err := json.Marshal(value)
	if err != nil {
		log.Printf("Error marshaling for cache [%s]: %v", key, err)
		return
	}

	if err := redisClient.Set(key, data, cacheDuration).Err(); err != nil {
		log.Printf("Error caching key [%s]: %v", key, err)
	}

}
func FetchFromCache(key string, target interface{}) bool {
	data, err := redisClient.Get(key).Bytes()
	if err != nil {
		return false
	}

	if err := json.Unmarshal(data, target); err != nil {
		log.Printf("Error unmarshaling cached data [%s]: %v", key, err)
		return false
	}

	return true
}
