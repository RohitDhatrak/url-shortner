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

func ctxServiceHandler(serviceFunc func(ctx *context.Context, w http.ResponseWriter, r *http.Request), ctx *context.Context) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		serviceFunc(ctx, w, r)
	}
}

func addValueToContext(ctx *context.Context, key, value interface{}) context.Context {
	return context.WithValue(*ctx, key, value)
}

func getDbFromContext(ctx *context.Context) *gorm.DB {
	return (*ctx).Value("db").(*gorm.DB)
}

func getUserFromContext(ctx *context.Context) *Users {
	user := (*ctx).Value("user")
	if user == nil {
		return nil
	}

	return user.(*Users)
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
	db := getDbFromContext(ctx)
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
	db := getDbFromContext(ctx)
	result := db.Create(urlShortener)

	if result.Error != nil {
		return &result.Error
	}

	return nil
}

func getUrlModel(ctx *context.Context, shortCode string) *UrlShortener {
	db := getDbFromContext(ctx)

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
	db := getDbFromContext(ctx)
	now := time.Now()
	newUrlShortener := UrlShortener{
		DeletedAt: &now,
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

func activateUrl(ctx *context.Context, shortCode string) error {
	db := getDbFromContext(ctx)

	result := db.Model(&UrlShortener{}).
		Where("short_code = ?", shortCode).
		Update("deleted_at", nil)

	if result.Error != nil {
		return result.Error
	}

	return nil
}

func getUserFromApiKeyIfExists(ctx *context.Context, apiKey string) *Users {
	db := getDbFromContext(ctx)
	var user Users

	result := db.Where("api_key = ?", apiKey).First(&user)
	if result.Error != nil {
		return nil
	}

	return &user
}

func getUrlsByUserId(ctx *context.Context, userId uint) []UrlShortener {
	db := getDbFromContext(ctx)
	var urls []UrlShortener
	db.Where("user_id = ?", userId).Find(&urls)
	return urls
}

func addressOf[T any](v T) *T {
	return &v
}

func newResponseWriter(w http.ResponseWriter) *CustomResponseWriter {
	return &CustomResponseWriter{
		ResponseWriter: w,
		headers:        make(http.Header),
		statusCode:     http.StatusOK,
	}
}

func (rw *CustomResponseWriter) Header() http.Header {
	return rw.headers
}

func (rw *CustomResponseWriter) WriteHeader(statusCode int) {
	rw.statusCode = statusCode
}

func (rw *CustomResponseWriter) Write(b []byte) (int, error) {
	rw.body = append(rw.body, b...)
	return len(b), nil
}

func (rw *CustomResponseWriter) Flush() {
	// Copy all headers to the original response writer
	for k, v := range rw.headers {
		for _, val := range v {
			rw.ResponseWriter.Header().Add(k, val)
		}
	}

	// Write the status code
	rw.ResponseWriter.WriteHeader(rw.statusCode)

	// Write the body
	rw.ResponseWriter.Write(rw.body)
}
