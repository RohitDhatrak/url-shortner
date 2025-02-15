package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

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

	http.HandleFunc("/health", health)
	http.HandleFunc("/shorten", CtxServiceHandler(shortenUrl, &ctx))
	http.HandleFunc("/shorten", CtxServiceHandler(deleteShortCode, &ctx))
	http.HandleFunc("/redirect", CtxServiceHandler(redirectToOriginalUrl, &ctx))

	port := ":8080"
	fmt.Printf("Server starting on port %s...\n", port)

	if err := http.ListenAndServe(port, nil); err != nil {
		log.Fatal("Error starting server: ", err)
	}

}

func NewDatabase(dbPath string) (*gorm.DB, error) {
	db, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{})
	if err != nil {
		return nil, err
	}

	db.AutoMigrate(&UrlShortener{})

	return db, nil
}
