package main

import (
	"crypto/sha256"
	"encoding/base64"

	"github.com/google/uuid"
)

func createShortCode(originalUrl string) string {
	shortCode := hashedUrl(originalUrl, 0)
	conflictFreeShortCode := createShortUrlWithRetry(originalUrl, shortCode, MAX_RETRIES)
	return conflictFreeShortCode
}

func hashedUrl(originalUrl string, additionalLength uint) string {
	HASH_TRIM_LENGTH := NORMAL_SHORT_CODE_LENGTH + additionalLength
	hash := sha256.Sum256([]byte(originalUrl))
	shortCode := base64.StdEncoding.EncodeToString(hash[:])

	return shortCode[:HASH_TRIM_LENGTH]
}

func createShortUrlWithRetry(ogUrl, shortCode string, retryCount uint) string {
	shortCodeExists := doesShortCodeExist(shortCode)
	if shortCodeExists {
		if retryCount > 0 {
			newShortCode := hashedUrl(ogUrl+uuid.New().String(), MAX_RETRIES-retryCount)
			return createShortUrlWithRetry(ogUrl, newShortCode, retryCount-1)
		} else {
			errMsg := "Error creating short url, max retry count exceded " + ogUrl
			panic(errMsg)
		}
	}

	return shortCode
}

func doesShortCodeExist(shortCode string) bool {
	var exists int
	query := "SELECT COUNT(*) FROM url_shorteners WHERE short_code = ?"
	err := db.QueryRow(query, shortCode).Scan(&exists)
	if err != nil {
		panic(err)
	}
	return exists > 0
}

func getOriginalUrl(shortCode string) string {
	var originalUrl string
	query := "SELECT original_url FROM url_shorteners WHERE short_code = ?"
	err := db.QueryRow(query, shortCode).Scan(&originalUrl)
	if err != nil {
		panic(err)
	}

	return originalUrl
}

func doesUrlExist(url string) bool {
	var exists int
	query := "SELECT COUNT(*) FROM url_shorteners WHERE original_url = ?"
	err := db.QueryRow(query, url).Scan(&exists)
	if err != nil {
		panic(err)
	}
	return exists > 0
}

func getShortCode(url string) string {
	var shortCode string
	query := "SELECT short_code FROM url_shorteners WHERE original_url = ?"
	err := db.QueryRow(query, url).Scan(&shortCode)
	if err != nil {
		panic(err)
	}
	return shortCode
}
