package main

import (
	"net/http"
	"sync/atomic"
	"time"

	"context"

	"gorm.io/gorm"
)

var counter uint64
var lastCounterEpochTimestamp int64

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

func createShortCode(ctx *context.Context, retryCount uint) string {
	if retryCount > MAX_RETRIES {
		errMsg := "Error creating short url, max retry count exceded"
		panic(errMsg)
	}

	// get current time in epoch starting from 1st Jan 2025
	currentEpochTime := getCustomEpochTime()

	// get an atomic counter to handle concurrent calls
	count := atomic.AddUint64(&counter, 1)

	// if the current epoch time is different from the last epoch time, reset the counter
	if currentEpochTime != lastCounterEpochTimestamp {
		atomic.StoreUint64(&counter, 0)
		lastCounterEpochTimestamp = currentEpochTime
	}

	// TODO: also add a service id if there are multiple instances of the service

	numbericShortCode := int64(count) + currentEpochTime
	shortCode := toBase36(numbericShortCode)

	shortCodeExists := doesShortCodeExist(ctx, shortCode)
	if shortCodeExists {
		return createShortCode(ctx, retryCount+1)
	}

	return shortCode
}

func getCustomEpochTime() int64 {
	customEpoch := time.Date(2025, time.January, 1, 0, 0, 0, 0, time.UTC)
	now := time.Now()
	return now.Unix() - customEpoch.Unix()
}

func toBase36(num int64) string {
	const base36Chars = "0123456789abcdefghijklmnopqrstuvwxyz"

	if num == 0 {
		return "0"
	}

	var result []byte
	for num > 0 {
		remainder := num % 36
		result = append([]byte{base36Chars[remainder]}, result...)
		num /= 36
	}

	return string(result)
}

func doesShortCodeExist(ctx *context.Context, shortCode string) bool {
	db := GetDbFromContext(ctx)
	var exists int64
	result := db.Model(&UrlShortener{}).
		Where("short_code = ?", shortCode).
		Where("deleted_at IS NULL").
		Where("expires_at IS NULL OR expires_at > ?", time.Now()).
		Count(&exists)

	if result.Error != nil {
		return false
	}

	return exists > 0
}

func insertUrl(ctx *context.Context, urlShortener *UrlShortener) *error {
	db := GetDbFromContext(ctx)
	result := db.Create(urlShortener)

	if result.Error != nil {
		return &result.Error
	}

	return nil
}

func getOriginalUrl(ctx *context.Context, shortCode string) string {
	db := GetDbFromContext(ctx)

	urlShortener := UrlShortener{}
	result := db.
		Where("deleted_at IS NULL").
		Where("expires_at IS NULL OR expires_at > ?", time.Now()).
		Find(&urlShortener, UrlShortener{ShortCode: shortCode})

	if result.Error != nil {
		return ""
	}

	newUrlShortener := UrlShortener{
		Views:      urlShortener.Views + 1,
		LastViewed: time.Now(),
	}

	result = db.Model(UrlShortener{}).
		Where(UrlShortener{
			ShortCode: urlShortener.ShortCode,
		}).Where("deleted_at IS NULL").Updates(newUrlShortener)

	if result.Error != nil {
		return ""
	}

	return urlShortener.OriginalUrl
}

func getUrlModel(ctx *context.Context, shortCode string) *UrlShortener {
	db := GetDbFromContext(ctx)

	urlShortener := UrlShortener{}
	result := db.
		Model(&UrlShortener{}).
		Where("short_code = ?", shortCode).
		Where("deleted_at IS NULL").
		Where("expires_at IS NULL OR expires_at > ?", time.Now()).
		First(&urlShortener)

	if result.Error != nil {
		return nil
	}

	return &urlShortener
}

func deleteUrl(ctx *context.Context, shortCode string) error {
	db := GetDbFromContext(ctx)
	newUrlShortener := UrlShortener{
		DeletedAt: time.Now(),
	}

	result := db.Model(UrlShortener{}).
		Where(UrlShortener{
			ShortCode: shortCode,
		}).Updates(newUrlShortener)

	if result.Error != nil {
		return result.Error
	}

	return nil
}

func getUserFromApiKeyIfExists(ctx *context.Context, apiKey string) *Users {
	db := GetDbFromContext(ctx)

	user := Users{}
	result := db.Find(&user, Users{ApiKey: apiKey})

	if result.Error != nil {
		return nil
	}

	return &user
}
