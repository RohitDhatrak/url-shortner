package main

import (
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"os"

	_ "github.com/mattn/go-sqlite3"
)

var db *sql.DB

func main() {
	err := os.MkdirAll("db", 0755)
	if err != nil {
		log.Fatal(err)
	}

	db, err = NewDatabase("db/database.sqlite")
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	http.HandleFunc("/health", health)
	http.HandleFunc("/shorten", shortenUrl)
	http.HandleFunc("/redirect", redirectToOriginalUrl)

	port := ":8080"
	fmt.Printf("Server starting on port %s...\n", port)

	if err := http.ListenAndServe(port, nil); err != nil {
		log.Fatal("Error starting server: ", err)
	}

}

func NewDatabase(dbPath string) (*sql.DB, error) {
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, err
	}

	err = db.Ping()
	if err != nil {
		return nil, err
	}

	err = createTables(db)
	if err != nil {
		return nil, err
	}

	return db, nil
}

func createTables(db *sql.DB) error {
	createTableSQL := `
    CREATE TABLE IF NOT EXISTS url_shorteners (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		original_url VARCHAR(2048) NOT NULL,
		short_code VARCHAR(255) UNIQUE NOT NULL,
		created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
		updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
		deleted_at TIMESTAMP
    );`

	_, err := db.Exec(createTableSQL)
	return err
}
