package main

import (
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"net/http"

	"context"

	"github.com/google/uuid"
)

func CtxServiceHandler(serviceFunc func(ctx *context.Context, w http.ResponseWriter, r *http.Request), ctx *context.Context) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		serviceFunc(ctx, w, r)
	}
}

func AddValueToContext(ctx *context.Context, key, value interface{}) context.Context {
	return context.WithValue(*ctx, key, value)
}

func GetDbFromContext(ctx *context.Context) *sql.DB {
	return (*ctx).Value("db").(*sql.DB)
}

func createShortCode(ctx *context.Context, originalUrl string) string {
	shortCode := hashedUrl(originalUrl, 0)
	conflictFreeShortCode := createShortUrlWithRetry(ctx, originalUrl, shortCode, MAX_RETRIES)
	return conflictFreeShortCode
}

func hashedUrl(originalUrl string, additionalLength uint) string {
	HASH_TRIM_LENGTH := NORMAL_SHORT_CODE_LENGTH + additionalLength
	hash := sha256.Sum256([]byte(originalUrl))
	shortCode := base64.StdEncoding.EncodeToString(hash[:])

	return shortCode[:HASH_TRIM_LENGTH]
}

func createShortUrlWithRetry(ctx *context.Context, ogUrl, shortCode string, retryCount uint) string {
	shortCodeExists := doesShortCodeExist(ctx, shortCode)
	if shortCodeExists {
		if retryCount > 0 {
			newShortCode := hashedUrl(ogUrl+uuid.New().String(), MAX_RETRIES-retryCount)
			return createShortUrlWithRetry(ctx, ogUrl, newShortCode, retryCount-1)
		} else {
			errMsg := "Error creating short url, max retry count exceded " + ogUrl
			panic(errMsg)
		}
	}

	return shortCode
}

func doesShortCodeExist(ctx *context.Context, shortCode string) bool {
	db := GetDbFromContext(ctx)
	var exists int
	query := "SELECT COUNT(*) FROM url_shorteners WHERE short_code = ?"
	err := db.QueryRow(query, shortCode).Scan(&exists)
	if err != nil {
		panic(err)
	}
	return exists > 0
}

func getOriginalUrl(ctx *context.Context, shortCode string) string {
	db := GetDbFromContext(ctx)
	var originalUrl string
	query := "SELECT original_url FROM url_shorteners WHERE short_code = ?"
	err := db.QueryRow(query, shortCode).Scan(&originalUrl)
	if err != nil {
		panic(err)
	}

	return originalUrl
}

func doesUrlExist(ctx *context.Context, url string) bool {
	db := GetDbFromContext(ctx)
	var exists int
	query := "SELECT COUNT(*) FROM url_shorteners WHERE original_url = ?"
	err := db.QueryRow(query, url).Scan(&exists)
	if err != nil {
		panic(err)
	}
	return exists > 0
}

func getShortCode(ctx *context.Context, url string) string {
	db := GetDbFromContext(ctx)
	var shortCode string
	query := "SELECT short_code FROM url_shorteners WHERE original_url = ?"
	err := db.QueryRow(query, url).Scan(&shortCode)
	if err != nil {
		panic(err)
	}
	return shortCode
}
