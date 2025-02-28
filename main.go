package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/gorilla/mux"
	_ "github.com/mattn/go-sqlite3"
)

func main() {
	err := os.MkdirAll("db", 0755)
	if err != nil {
		log.Fatal(err)
	}

	db, err := NewDatabase("db/database.sqlite")
	if err != nil {
		log.Fatal(err)
	}

	ctx := context.Background()
	ctx = AddValueToContext(&ctx, "db", db)

	r := mux.NewRouter()
	r.Use(loggingMiddleware(&ctx))

	r.HandleFunc("/health", CtxServiceHandler(health, &ctx)).Methods("GET")
	r.HandleFunc("/shorten", CtxServiceHandler(shortenUrl, &ctx)).Methods("POST")
	r.HandleFunc("/shorten", CtxServiceHandler(deleteShortCode, &ctx)).Methods("DELETE")
	r.HandleFunc("/shorten/bulk", CtxServiceHandler(shortenUrlBulk, &ctx)).Methods("POST")
	r.HandleFunc("/shorten/edit", CtxServiceHandler(editUrl, &ctx)).Methods("PUT")
	r.HandleFunc("/redirect", CtxServiceHandler(redirectToOriginalUrl, &ctx)).Methods("GET")
	r.HandleFunc("/user/urls", CtxServiceHandler(getUserUrls, &ctx)).Methods("GET")

	port := ":8080"
	fmt.Printf("Server starting on port %s...\n", port)

	if err := http.ListenAndServe(port, r); err != nil {
		log.Fatal("Error starting server: ", err)
	}

}

func loggingMiddleware(ctx *context.Context) mux.MiddlewareFunc {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			db := GetDbFromContext(ctx)
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
