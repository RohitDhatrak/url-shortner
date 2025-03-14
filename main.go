package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/go-redis/redis"
	"github.com/gorilla/mux"
	_ "github.com/mattn/go-sqlite3"
)

var (
	blockedAPIKeys    map[string]bool
	blocklistMutex    sync.RWMutex
	lastBlocklistLoad time.Time
)

type CustomResponseWriter struct {
	http.ResponseWriter
	headers    http.Header
	body       []byte
	statusCode int
}

var redisClient *redis.Client

func initRedis() {
	redisClient = redis.NewClient(&redis.Options{
		Addr:     "localhost:6379", // Redis server address
		Password: "",               // No password by default
		DB:       0,                // Default DB
	})
}

func main() {
	err := os.MkdirAll("db", 0755)
	if err != nil {
		log.Fatal(err)
	}

	db, err := NewDatabase("db/database.sqlite")
	if err != nil {
		log.Fatal(err)
	}
	initRedis()

	ctx := context.Background()
	ctx = addValueToContext(&ctx, "db", db)

	unauthenticatedRouter := mux.NewRouter()
	unauthenticatedRouter.Use(responseTimeMiddleware())
	unauthenticatedRouter.Use(loggingMiddleware(&ctx))
	unauthenticatedRouter.Use(blocklistMiddleware())
	unauthenticatedRouter.Use(ipRateLimitMiddleware())

	authenticatedRouter := unauthenticatedRouter.PathPrefix("").Subrouter()
	authenticatedRouter.Use(apiKeyMiddleware(&ctx))

	pricingRouter := authenticatedRouter.PathPrefix("").Subrouter()
	pricingRouter.Use(pricingPlanMiddleware(&ctx))

	unauthenticatedRouter.HandleFunc("/health", ctxServiceHandler(health, &ctx)).Methods("GET")
	unauthenticatedRouter.HandleFunc("/shorten", ctxServiceHandler(shortenUrl, &ctx)).Methods("POST")
	unauthenticatedRouter.HandleFunc("/redirect", ctxServiceHandler(redirectToOriginalUrl, &ctx)).Methods("GET")

	authenticatedRouter.HandleFunc("/shorten", ctxServiceHandler(deleteShortCode, &ctx)).Methods("DELETE")
	authenticatedRouter.HandleFunc("/shorten", ctxServiceHandler(editUrl, &ctx)).Methods("PUT")
	authenticatedRouter.HandleFunc("/user/urls", ctxServiceHandler(getUserUrls, &ctx)).Methods("GET")

	pricingRouter.HandleFunc("/shorten/bulk", ctxServiceHandler(shortenUrlBulk, &ctx)).Methods("POST")

	port := ":8080"
	fmt.Printf("Server starting on port %s...\n", port)

	if err := http.ListenAndServe(port, unauthenticatedRouter); err != nil {
		log.Fatal("Error starting server: ", err)
	}
}

func responseTimeMiddleware() mux.MiddlewareFunc {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			wrapper := newResponseWriter(w)

			startTime := time.Now()

			next.ServeHTTP(wrapper, r)

			elapsedTime := time.Since(startTime)

			wrapper.Header().Set("X-Response-Time", elapsedTime.String())

			wrapper.Flush()
		})
	}
}

func loggingMiddleware(ctx *context.Context) mux.MiddlewareFunc {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			db := getDbFromContext(ctx)
			timestamp := time.Now()
			logRequest := LogRequests{
				Timestamp: timestamp,
				Method:    r.Method,
				Url:       r.URL.Path,
				UserAgent: r.UserAgent(),
				IpAddress: r.RemoteAddr,
			}
			db.Create(&logRequest)
			next.ServeHTTP(w, r)
			log.Printf("%s %s %v", r.Method, r.URL.Path, time.Since(timestamp))
		})
	}
}

func loadBlocklist() error {
	blocklistMutex.Lock()
	defer blocklistMutex.Unlock()

	if time.Since(lastBlocklistLoad) < 5*time.Minute && blockedAPIKeys != nil {
		return nil
	}

	data, err := os.ReadFile("blacklist.csv")
	if err != nil {
		if os.IsNotExist(err) {
			blockedAPIKeys = make(map[string]bool)
			lastBlocklistLoad = time.Now()
			return nil
		}
		return err
	}

	newBlocklist := make(map[string]bool)
	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		apiKey := strings.TrimSpace(line)
		if apiKey != "" {
			newBlocklist[apiKey] = true
		}
	}

	blockedAPIKeys = newBlocklist
	lastBlocklistLoad = time.Now()
	return nil
}

func blocklistMiddleware() mux.MiddlewareFunc {
	if err := loadBlocklist(); err != nil {
		log.Printf("Warning: Failed to load API key blocklist: %v", err)
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			apiKey := strings.TrimSpace(r.Header.Get("X-API-Key"))

			if apiKey == "" {
				next.ServeHTTP(w, r)
				return
			}

			if err := loadBlocklist(); err != nil {
				http.Error(w, "Something went wrong", http.StatusInternalServerError)
			}

			blocklistMutex.RLock()
			isBlocked := blockedAPIKeys[apiKey]
			blocklistMutex.RUnlock()

			if isBlocked {
				http.Error(w, "API key is blocked", http.StatusUnauthorized)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

func ipRateLimitMiddleware() mux.MiddlewareFunc {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ip := r.RemoteAddr

			var redisKey string
			var rateLimit int64

			if r.URL.Path == "/redirect" {
				redisKey = "redirect:" + ip
				rateLimit = 50
			} else if r.URL.Path == "/shorten" {
				redisKey = "shorten:" + ip
				rateLimit = 10
			} else {
				redisKey = "default:" + ip
				rateLimit = 100
			}

			count, err := redisClient.Incr(redisKey).Result()
			if err != nil {
				http.Error(w, "Error incrementing request count", http.StatusInternalServerError)
				return
			}

			if count == 1 {
				var expiry time.Duration
				if r.URL.Path == "/redirect" || r.URL.Path == "/shorten" {
					expiry = 1 * time.Second
				} else {
					expiry = 1 * time.Minute
				}

				err = redisClient.Expire(redisKey, expiry).Err()
				if err != nil {
					http.Error(w, "Error setting expiry", http.StatusInternalServerError)
					return
				}
			}

			if count > rateLimit {
				http.Error(w, "Too many requests", http.StatusTooManyRequests)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

func apiKeyMiddleware(ctx *context.Context) mux.MiddlewareFunc {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			apiKey := r.Header.Get("X-API-Key")

			if apiKey == "" {
				http.Error(w, "Invalid API key", http.StatusUnauthorized)
				return
			}

			user := getUserFromApiKeyIfExists(ctx, apiKey)

			if user == nil {
				http.Error(w, "Invalid API key", http.StatusUnauthorized)
				return
			}

			*ctx = addValueToContext(ctx, "user", user)

			next.ServeHTTP(w, r)
		})
	}
}

func pricingPlanMiddleware(ctx *context.Context) mux.MiddlewareFunc {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			user := getUserFromContext(ctx)

			if user.Tier != "enterprise" {
				http.Error(w, "You are not authorized to access this resource", http.StatusForbidden)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

func NewDatabase(dbPath string) (*gorm.DB, error) {
	db, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{})
	if err != nil {
		return nil, err
	}

	db.AutoMigrate(&UrlShortener{})
	db.AutoMigrate(&Users{})
	db.AutoMigrate(&LogRequests{})

	return db, nil
}
