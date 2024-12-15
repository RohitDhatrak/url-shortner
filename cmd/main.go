package main

import (
	"fmt"

	"time"

	"crypto/sha256"
	"encoding/base64"
	"errors"

	"github.com/google/uuid"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

const MAX_RETRIES = 3
const NORMAL_SHORT_CODE_LENGTH = 8

func main() {
	db := initDB()
	const NO_OF_ENTRIES = 1000

	fmt.Println("Time started:", time.Now().Format("15:04:05"))
	addNEntries(db, NO_OF_ENTRIES)
	fmt.Println("Time ended:", time.Now().Format("15:04:05"))
}

func initDB() *gorm.DB {
	dsn := "user=postgres dbname=vyson_db sslmode=disable"
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		panic("Failed to connect database")
	}
	return db
}

func addNEntries(db *gorm.DB, noOfEntries int) {

	for i := 0; i < noOfEntries; i++ {
		originalUrl := fmt.Sprintf("https://www.example.com/%s", uuid.New().String())
		shortCode := hashedUrl(originalUrl, 0)
		createShortUrlWithRetry(db, originalUrl, shortCode, MAX_RETRIES)
	}
}

func hashedUrl(originalUrl string, additionalLength uint) string {
	HASH_TRIM_LENGTH := NORMAL_SHORT_CODE_LENGTH + additionalLength
	hash := sha256.Sum256([]byte(originalUrl))
	shortCode := base64.StdEncoding.EncodeToString(hash[:])

	return shortCode[:HASH_TRIM_LENGTH]
}

func createShortUrlWithRetry(db *gorm.DB, ogUrl, shortCode string, retryCount uint) {
	shortCodeExists := doesShortCodeExist(db, shortCode)
	if shortCodeExists {
		if retryCount > 0 {
			newShortCode := hashedUrl(ogUrl+uuid.New().String(), MAX_RETRIES-retryCount)
			createShortUrlWithRetry(db, ogUrl, newShortCode, retryCount-1)
		} else {
			fmt.Println("Error creating short url, max retry count exceded ", ogUrl)
		}
		return
	}

	result := db.Create(&UrlShortener{OriginalUrl: ogUrl, ShortCode: shortCode})
	if result.Error != nil {
		fmt.Println("Error creating short url ", ogUrl, result.Error)
	}
}

func doesShortCodeExist(db *gorm.DB, shortCode string) bool {
	model := UrlShortener{}
	result := db.Model(UrlShortener{}).First(&model, UrlShortener{ShortCode: shortCode})
	if result.Error != nil && errors.Is(result.Error, gorm.ErrRecordNotFound) {
		return false
	}

	return true
}
