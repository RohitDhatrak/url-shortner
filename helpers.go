package main

import (
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"net/http"

	"context"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

func CtxServiceHandler(serviceFunc func(ctx *context.Context, w http.ResponseWriter, r *http.Request), ctx *context.Context) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		serviceFunc(ctx, w, r)
	}
}

func AddValueToContext(ctx *context.Context, key, value interface{}) context.Context {
	return context.WithValue(*ctx, key, value)
}

func GetDbFromContext(ctx *context.Context) *gorm.DB {
	return (*ctx).Value("db").(*gorm.DB)
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
	var exists int64
	db.Find(&UrlShortener{ShortCode: shortCode}).Count(&exists)

	fmt.Println("exists", exists)

	return exists > 0
}

func insertUrl(ctx *context.Context, originalUrl, shortCode string) {
	db := GetDbFromContext(ctx)
	result := db.Create(&UrlShortener{OriginalUrl: originalUrl, ShortCode: shortCode})

	if result.Error != nil {
		panic("Error inserting url into db")
	}
}

func getOriginalUrl(ctx *context.Context, shortCode string) string {
	db := GetDbFromContext(ctx)

	urlShortener := UrlShortener{}
	result := db.Find(&urlShortener, UrlShortener{ShortCode: shortCode})

	if result.Error != nil {
		return ""
	}

	return urlShortener.OriginalUrl
}

func doesUrlExist(ctx *context.Context, url string) bool {
	db := GetDbFromContext(ctx)
	var exists int64
	result := db.Find(&UrlShortener{OriginalUrl: url}).Count(&exists)

	if result.Error != nil {
		return false
	}

	return exists > 0
}

func getShortCode(ctx *context.Context, url string) string {
	db := GetDbFromContext(ctx)

	urlShortener := UrlShortener{}
	result := db.First(&urlShortener, UrlShortener{OriginalUrl: url})

	if result.Error != nil {
		return ""
	}

	return urlShortener.ShortCode
}

func deleteUrl(ctx *context.Context, shortCode string) error {
	db := GetDbFromContext(ctx)
	result := db.Delete(&UrlShortener{}, "short_code = ?", shortCode)

	if result.Error != nil {
		return result.Error
	}

	return nil
}
