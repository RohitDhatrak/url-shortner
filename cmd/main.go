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
	const NO_OF_ENTRIES = 10_00_00_000       // 100M
	const NO_OF_TIMES_QUERY = 1_00_00_00_000 // 1B
	startedTime := time.Now().Format("15:04:05")

	addNEntries(db, NO_OF_ENTRIES)
	// queryNTimes(db, NO_OF_TIMES_QUERY)

	fmt.Println("Time started:", startedTime)
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

func queryNTimes(db *gorm.DB, noOfTimesToQuery int) {
	shortCodes := []string{"OEWpcwvi", "ST2Xo4eP", "mc24YGya", "yHkf4oXB", "AwibCalY"}
	models := []UrlShortener{}

	for i := 0; i < noOfTimesToQuery; i++ {
		result := db.Where("short_code IN ?", shortCodes).Find(&models)
		if result.Error != nil {
			panic(result.Error)
		}
	}

	fmt.Println(models)
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
			errMsg := "Error creating short url, max retry count exceded " + ogUrl
			panic(errMsg)
		}
		return
	}

	result := db.Create(&UrlShortener{OriginalUrl: ogUrl, ShortCode: shortCode})
	if result.Error != nil {
		panic(result.Error)
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
