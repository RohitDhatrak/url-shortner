package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

func InitTest() *gorm.DB {
	db, err := NewDatabase("db/database.sqlite")
	if err != nil {
		log.Fatal(err)
	}

	return db
}

func TestShortenAndRedirect(t *testing.T) {
	db := InitTest()

	ctx := context.Background()
	ctx = addValueToContext(&ctx, "db", db)

	// Simulate a POST request with a URL in the request body
	originalUrl := "http://example.com"
	shortenReqBody := strings.NewReader(`{"url": "` + originalUrl + `"}`)
	shortenReq, err := http.NewRequest("POST", "/shorten", shortenReqBody)
	if err != nil {
		t.Fatal(err)
	}
	shortenReq.Header.Set("Content-Type", "application/json")

	// Create a ResponseRecorder to record the response
	shortenRR := httptest.NewRecorder()
	handler := http.HandlerFunc(ctxServiceHandler(shortenUrl, &ctx))

	// Serve the HTTP request
	handler.ServeHTTP(shortenRR, shortenReq)

	// Check the status code
	if status := shortenRR.Code; status != http.StatusCreated {
		t.Errorf("handler returned wrong status code: got %v want %v", status, http.StatusCreated)
	}

	// Extract the shortCode from the response body
	var response map[string]string
	if err := json.NewDecoder(shortenRR.Body).Decode(&response); err != nil {
		t.Fatal("Failed to decode response body")
	}
	shortCode, exists := response["short_code"]
	if !exists {
		t.Fatal("short_code not found in response")
	}

	// Simulate a GET request to the redirect endpoint
	redirectReq, err := http.NewRequest("GET", "/redirect?code="+shortCode, nil)
	if err != nil {
		t.Fatal(err)
	}

	// Create a new ResponseRecorder for the redirect request
	redirectRR := httptest.NewRecorder()
	redirectHandler := http.HandlerFunc(ctxServiceHandler(redirectToOriginalUrl, &ctx))

	// Serve the redirect request
	redirectHandler.ServeHTTP(redirectRR, redirectReq)

	// Check if the redirect happened to the correct URL
	if redirectRR.Code != http.StatusTemporaryRedirect {
		t.Errorf("redirect handler returned wrong status code: got %v want %v", redirectRR.Code, http.StatusTemporaryRedirect)
	}

	// Check the Location header for the correct URL
	if location := redirectRR.Header().Get("Location"); location != originalUrl {
		t.Errorf("redirect handler returned wrong location: got %v want %v", location, originalUrl)
	}
}

func TestRedirectNonExistentShortCode(t *testing.T) {
	db := InitTest()

	ctx := context.Background()
	ctx = addValueToContext(&ctx, "db", db)

	// Simulate a GET request to the redirect endpoint with a non-existent short code
	nonExistentShortCode := "nonexistent123"
	redirectReq, err := http.NewRequest("GET", "/redirect?code="+nonExistentShortCode, nil)
	if err != nil {
		t.Fatal(err)
	}

	// Create a new ResponseRecorder for the redirect request
	redirectRR := httptest.NewRecorder()
	redirectHandler := http.HandlerFunc(ctxServiceHandler(redirectToOriginalUrl, &ctx))

	// Serve the redirect request
	redirectHandler.ServeHTTP(redirectRR, redirectReq)

	// Check if the response status code is 404
	if status := redirectRR.Code; status != http.StatusNotFound {
		t.Errorf("handler returned wrong status code: got %v want %v", status, http.StatusNotFound)
	}
}

func TestShortenEmptyUrl(t *testing.T) {
	db := InitTest()

	ctx := context.Background()
	ctx = addValueToContext(&ctx, "db", db)

	// Simulate a POST request with an empty URL in the request body
	shortenReqBody := strings.NewReader(`{"url": ""}`)
	shortenReq, err := http.NewRequest("POST", "/shorten", shortenReqBody)
	if err != nil {
		t.Fatal(err)
	}
	shortenReq.Header.Set("Content-Type", "application/json")

	// Create a ResponseRecorder to record the response
	shortenRR := httptest.NewRecorder()
	handler := http.HandlerFunc(ctxServiceHandler(shortenUrl, &ctx))

	// Serve the HTTP request
	handler.ServeHTTP(shortenRR, shortenReq)

	// Check if the status code is BadRequest (400)
	if status := shortenRR.Code; status != http.StatusBadRequest {
		t.Errorf("handler returned wrong status code: got %v want %v",
			status, http.StatusBadRequest)
	}

	// Check the error message
	expected := "URL is required\n"
	if shortenRR.Body.String() != expected {
		t.Errorf("handler returned unexpected body: got %v want %v",
			shortenRR.Body.String(), expected)
	}
}

func TestSameUrlReturnsDifferentShortCodes(t *testing.T) {
	db := InitTest()
	ctx := context.Background()
	ctx = addValueToContext(&ctx, "db", db)

	// Make multiple requests with the same URL
	originalUrl := "http://example.com"
	shortCodes := make(map[string]bool)
	numRequests := 5

	for i := 0; i < numRequests; i++ {
		shortenReqBody := strings.NewReader(`{"url": "` + originalUrl + `"}`)
		shortenReq, err := http.NewRequest("POST", "/shorten", shortenReqBody)
		if err != nil {
			t.Fatal(err)
		}
		shortenReq.Header.Set("Content-Type", "application/json")

		shortenRR := httptest.NewRecorder()
		handler := http.HandlerFunc(ctxServiceHandler(shortenUrl, &ctx))
		handler.ServeHTTP(shortenRR, shortenReq)

		// Check status code
		if status := shortenRR.Code; status != http.StatusCreated {
			t.Errorf("handler returned wrong status code: got %v want %v",
				status, http.StatusCreated)
		}

		// Extract short code from response
		var response map[string]string
		if err := json.NewDecoder(shortenRR.Body).Decode(&response); err != nil {
			t.Fatal("Failed to decode response body")
		}
		shortCode := response["short_code"]

		// Check if this short code was already seen
		if shortCodes[shortCode] {
			t.Errorf("Duplicate short code generated: %s", shortCode)
		}
		shortCodes[shortCode] = true
	}

	// Verify we got the expected number of unique short codes
	if len(shortCodes) != numRequests {
		t.Errorf("Expected %d unique short codes, got %d",
			numRequests, len(shortCodes))
	}
}

func TestShortenUrlWithApiKey(t *testing.T) {
	db := InitTest()
	ctx := context.Background()
	ctx = addValueToContext(&ctx, "db", db)

	// First create a test user

	testUser := &Users{
		Email:     "tesdfdsst@example.com",
		Name:      addressOf("Test User"),
		ApiKey:    "test_api_key_123dfdd",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	result := db.Create(testUser)
	if result.Error != nil {
		t.Fatal("Failed to create test user:", result.Error)
	}

	// Create a URL with the API key
	originalUrl := "http://example.com"
	shortenReqBody := strings.NewReader(`{"url": "` + originalUrl + `"}`)
	shortenReq, err := http.NewRequest("POST", "/shorten", shortenReqBody)
	if err != nil {
		t.Fatal(err)
	}

	// Set headers
	shortenReq.Header.Set("Content-Type", "application/json")
	shortenReq.Header.Set("X-API-Key", testUser.ApiKey)

	// Create response recorder and handle request
	shortenRR := httptest.NewRecorder()
	handler := http.HandlerFunc(ctxServiceHandler(shortenUrl, &ctx))
	handler.ServeHTTP(shortenRR, shortenReq)

	// Check status code
	if status := shortenRR.Code; status != http.StatusCreated {
		t.Errorf("handler returned wrong status code: got %v want %v",
			status, http.StatusCreated)
	}

	// Get the short code from response
	var response map[string]string
	if err := json.NewDecoder(shortenRR.Body).Decode(&response); err != nil {
		t.Fatal("Failed to decode response body")
	}
	shortCode := response["short_code"]

	// Verify the URL was created with the correct user ID
	var urlShortener UrlShortener
	result = db.Where("short_code = ?", shortCode).First(&urlShortener)
	if result.Error != nil {
		t.Fatal("Failed to find created URL:", result.Error)
	}

	// Check if the user ID matches
	if *urlShortener.UserId != testUser.Id {
		t.Errorf("URL not linked to correct user: got user ID %v, want %v",
			urlShortener.UserId, testUser.Id)
	}

	// Clean up
	db.Unscoped().Delete(&testUser)
	db.Unscoped().Delete(&urlShortener)
}

func TestDeleteShortCodeAuthorization(t *testing.T) {
	db := InitTest()
	ctx := context.Background()
	ctx = addValueToContext(&ctx, "db", db)

	// Create two test users
	user1 := &Users{
		Email:     "test-user1@example.com",
		Name:      addressOf("Test User One"),
		ApiKey:    "test_api_key_user1",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	user2 := &Users{
		Email:     "test-user2@example.com",
		Name:      addressOf("Test User Two"),
		ApiKey:    "test_api_key_user2",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	db.Create(user1)
	db.Create(user2)

	user1 = getUserFromApiKeyIfExists(&ctx, user1.ApiKey)
	user2 = getUserFromApiKeyIfExists(&ctx, user2.ApiKey)

	// Create a URL with user1's API key
	originalUrl := "http://example.com"
	shortenReqBody := strings.NewReader(`{"url": "` + originalUrl + `"}`)
	shortenReq, err := http.NewRequest("POST", "/shorten", shortenReqBody)
	if err != nil {
		t.Fatal(err)
	}
	shortenReq.Header.Set("Content-Type", "application/json")
	shortenReq.Header.Set("X-API-Key", user1.ApiKey)
	ctx = addValueToContext(&ctx, "user", user1)

	shortenRR := httptest.NewRecorder()
	handler := http.HandlerFunc(ctxServiceHandler(shortenUrl, &ctx))
	handler.ServeHTTP(shortenRR, shortenReq)

	var response map[string]string
	if err := json.NewDecoder(shortenRR.Body).Decode(&response); err != nil {
		t.Fatal("Failed to decode response body")
	}
	shortCode := response["short_code"]

	// Test 1: Try to delete with user2's API key (should fail)
	ctx = addValueToContext(&ctx, "user", user2)
	deleteReq, _ := http.NewRequest("DELETE", "/shorten?code="+shortCode, nil)
	deleteReq.Header.Set("X-API-Key", user2.ApiKey)
	deleteRR := httptest.NewRecorder()
	deleteHandler := http.HandlerFunc(ctxServiceHandler(deleteShortCode, &ctx))
	deleteHandler.ServeHTTP(deleteRR, deleteReq)

	if status := deleteRR.Code; status != http.StatusForbidden {
		t.Errorf("Expected status forbidden for unauthorized user, got %v", status)
	}

	// Test 2: Delete with user1's API key (should succeed)
	ctx = addValueToContext(&ctx, "user", user1)
	deleteReq, _ = http.NewRequest("DELETE", "/shorten?code="+shortCode, nil)
	deleteReq.Header.Set("X-API-Key", user1.ApiKey)
	deleteRR = httptest.NewRecorder()
	deleteHandler = http.HandlerFunc(ctxServiceHandler(deleteShortCode, &ctx))
	deleteHandler.ServeHTTP(deleteRR, deleteReq)

	if status := deleteRR.Code; status != http.StatusOK {
		t.Errorf("Expected status OK for authorized user, got %v", status)
	}

	// Clean up
	db.Unscoped().Delete(user1)
	db.Unscoped().Delete(user2)
}

func TestHelperDeletionAndExpiry(t *testing.T) {
	db := InitTest()
	ctx := context.Background()
	ctx = addValueToContext(&ctx, "db", db)

	shortCode := "2wk9m"
	exists := doesShortCodeExist(&ctx, shortCode)

	if exists {
		t.Errorf("Expected short code to not exist, got %v", exists)
	}
}

func TestUrlExpiration(t *testing.T) {
	db := InitTest()
	ctx := context.Background()
	ctx = addValueToContext(&ctx, "db", db)

	// Create a URL that expires in 2 seconds
	originalUrl := "http://example.com"
	expiresAt := time.Now().Add(2 * time.Second).Format(time.RFC3339)
	shortenReqBody := strings.NewReader(fmt.Sprintf(`{"url": "%s", "expires_at": "%s"}`, originalUrl, expiresAt))

	shortenReq, err := http.NewRequest("POST", "/shorten", shortenReqBody)
	if err != nil {
		t.Fatal(err)
	}
	shortenReq.Header.Set("Content-Type", "application/json")

	shortenRR := httptest.NewRecorder()
	handler := http.HandlerFunc(ctxServiceHandler(shortenUrl, &ctx))
	handler.ServeHTTP(shortenRR, shortenReq)

	// Check if URL was created successfully
	if status := shortenRR.Code; status != http.StatusCreated {
		t.Errorf("handler returned wrong status code: got %v want %v",
			status, http.StatusCreated)
	}

	var response map[string]string
	if err := json.NewDecoder(shortenRR.Body).Decode(&response); err != nil {
		t.Fatal("Failed to decode response body")
	}
	shortCode := response["short_code"]

	// Verify URL exists before expiration
	exists := doesShortCodeExist(&ctx, shortCode)
	if !exists {
		t.Error("URL should exist before expiration")
	}

	// Wait for URL to expire
	time.Sleep(3 * time.Second)

	// Verify URL doesn't exist after expiration
	exists = doesShortCodeExist(&ctx, shortCode)
	if exists {
		t.Error("URL should not exist after expiration")
	}

	// Try to access the expired URL
	redirectReq, _ := http.NewRequest("GET", "/redirect?code="+shortCode, nil)
	redirectRR := httptest.NewRecorder()
	redirectHandler := http.HandlerFunc(ctxServiceHandler(redirectToOriginalUrl, &ctx))
	redirectHandler.ServeHTTP(redirectRR, redirectReq)

	if status := redirectRR.Code; status != http.StatusNotFound {
		t.Errorf("Expected status not found for expired URL, got %v", status)
	}
}

func TestCustomUrlShortening(t *testing.T) {
	db := InitTest()
	ctx := context.Background()
	ctx = addValueToContext(&ctx, "db", db)

	// Test 1: Create a URL with custom short code
	originalUrl := "http://example.com"
	customUrl := uuid.New().String()[:5]
	shortenReqBody := strings.NewReader(fmt.Sprintf(`{"url": "%s", "custom_url": "%s"}`, originalUrl, customUrl))

	shortenReq, err := http.NewRequest("POST", "/shorten", shortenReqBody)
	if err != nil {
		t.Fatal(err)
	}
	shortenReq.Header.Set("Content-Type", "application/json")

	shortenRR := httptest.NewRecorder()
	handler := http.HandlerFunc(ctxServiceHandler(shortenUrl, &ctx))
	handler.ServeHTTP(shortenRR, shortenReq)

	// Check if URL was created successfully
	if status := shortenRR.Code; status != http.StatusCreated {
		t.Errorf("handler returned wrong status code: got %v want %v",
			status, http.StatusCreated)
	}

	var response map[string]string
	if err := json.NewDecoder(shortenRR.Body).Decode(&response); err != nil {
		t.Fatal("Failed to decode response body")
	}

	// Verify the returned short code matches our custom URL
	if response["short_code"] != customUrl {
		t.Errorf("Expected custom URL %s, got %s", customUrl, response["short_code"])
	}

	// Test 2: Try to create another URL with the same custom short code
	shortenReq, _ = http.NewRequest("POST", "/shorten", shortenReqBody)
	shortenReq.Header.Set("Content-Type", "application/json")
	shortenRR = httptest.NewRecorder()
	handler.ServeHTTP(shortenRR, shortenReq)

	// Should get a BadRequest status
	if status := shortenRR.Code; status != http.StatusBadRequest {
		t.Errorf("Expected status BadRequest for duplicate custom URL, got %v", status)
	}

	// Test 3: Verify the URL works through redirection
	redirectReq, _ := http.NewRequest("GET", "/redirect?code="+customUrl, nil)
	redirectRR := httptest.NewRecorder()
	redirectHandler := http.HandlerFunc(ctxServiceHandler(redirectToOriginalUrl, &ctx))
	redirectHandler.ServeHTTP(redirectRR, redirectReq)

	if status := redirectRR.Code; status != http.StatusTemporaryRedirect {
		t.Errorf("Expected redirect status, got %v", status)
	}

	if location := redirectRR.Header().Get("Location"); location != originalUrl {
		t.Errorf("Expected redirect to %s, got %s", originalUrl, location)
	}

	// Clean up
	deleteUrl(&ctx, customUrl)
}

func TestShortenUrlBulk(t *testing.T) {
	db := InitTest()
	ctx := context.Background()
	ctx = addValueToContext(&ctx, "db", db)

	user1 := &Users{
		Email:     uuid.New().String()[:5] + "@example.com",
		Name:      addressOf("Test User One"),
		ApiKey:    uuid.New().String()[:5],
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		Tier:      "enterprise",
	}
	db.Create(user1)

	user1 = getUserFromApiKeyIfExists(&ctx, user1.ApiKey)

	// Test case 1: Successful bulk URL shortening
	reqBody := strings.NewReader(`{
		"urls": [
			{"url": "http://example1.com"},
			{"url": "http://example2.com"},
			{"url": "http://example3.com"},
			{"url": "http://example4.com", "expires_at": "2025-01-01T00:00:00Z"}
		]
	}`)

	req, err := http.NewRequest("POST", "/shorten/bulk", reqBody)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-API-Key", user1.ApiKey)
	ctx = addValueToContext(&ctx, "user", user1)

	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(ctxServiceHandler(shortenUrlBulk, &ctx))
	handler.ServeHTTP(rr, req)

	// Check status code
	if status := rr.Code; status != http.StatusCreated {
		t.Errorf("handler returned wrong status code: got %v want %v",
			status, http.StatusCreated)
	}

	// Parse response
	var response map[string]string
	if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
		t.Fatal("Failed to decode response body")
	}

	// Verify short codes were returned
	shortCodesStr := response["short_codes"]
	var shortCodes []string
	if err := json.Unmarshal([]byte(shortCodesStr), &shortCodes); err != nil {
		fmt.Println(shortCodesStr)
		t.Fatal("Failed to parse short codes")
	}

	if len(shortCodes) != 4 {
		t.Errorf("Expected 4 short codes, got %d", len(shortCodes))
	}

	// Test case 2: Duplicate custom URLs
	reqBody = strings.NewReader(`{
		"urls": [
			{"url": "http://example1.com", "custom_url": "custom123"},
			{"url": "http://example2.com", "custom_url": "custom123"}
		]
	}`)

	req, _ = http.NewRequest("POST", "/shorten/bulk", reqBody)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-API-Key", user1.ApiKey)
	rr = httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusBadRequest {
		t.Errorf("handler should return BadRequest for duplicate custom URLs: got %v want %v",
			status, http.StatusBadRequest)
	}

	// Test case 3: Empty URL in batch
	reqBody = strings.NewReader(`{
		"urls": [
			{"url": "http://example1.com"},
			{"url": ""}
		]
	}`)

	req, _ = http.NewRequest("POST", "/shorten/bulk", reqBody)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-API-Key", user1.ApiKey)
	rr = httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusBadRequest {
		t.Errorf("handler should return BadRequest for empty URL: got %v want %v",
			status, http.StatusBadRequest)
	}

	// Clean up
	db.Unscoped().Delete(user1)
}

func TestActivateUrl(t *testing.T) {
	db := InitTest()
	ctx := context.Background()
	ctx = addValueToContext(&ctx, "db", db)

	activateUrl(&ctx, "194d5")
}

func TestDeleteUrl(t *testing.T) {
	db := InitTest()
	ctx := context.Background()
	ctx = addValueToContext(&ctx, "db", db)

	deleteUrl(&ctx, "194d5")
}

func TestPasswordProtectedUrl(t *testing.T) {
	db := InitTest()
	ctx := context.Background()
	ctx = addValueToContext(&ctx, "db", db)

	// Create a URL with password protection
	originalUrl := "http://example.com"
	password := "secretpass123"
	shortenReqBody := strings.NewReader(fmt.Sprintf(`{
		"url": "%s",
		"password": "%s"
	}`, originalUrl, password))

	shortenReq, err := http.NewRequest("POST", "/shorten", shortenReqBody)
	if err != nil {
		t.Fatal(err)
	}
	shortenReq.Header.Set("Content-Type", "application/json")

	shortenRR := httptest.NewRecorder()
	handler := http.HandlerFunc(ctxServiceHandler(shortenUrl, &ctx))
	handler.ServeHTTP(shortenRR, shortenReq)

	// Check if URL was created successfully
	if status := shortenRR.Code; status != http.StatusCreated {
		t.Errorf("handler returned wrong status code: got %v want %v",
			status, http.StatusCreated)
	}

	var response map[string]string
	if err := json.NewDecoder(shortenRR.Body).Decode(&response); err != nil {
		t.Fatal("Failed to decode response body")
	}
	shortCode := response["short_code"]

	// Test 1: Try to access URL without password
	redirectReq, _ := http.NewRequest("GET", "/redirect?code="+shortCode, nil)
	redirectRR := httptest.NewRecorder()
	redirectHandler := http.HandlerFunc(ctxServiceHandler(redirectToOriginalUrl, &ctx))
	redirectHandler.ServeHTTP(redirectRR, redirectReq)

	if status := redirectRR.Code; status != http.StatusBadRequest {
		t.Errorf("handler should return BadRequest when password is missing: got %v want %v",
			status, http.StatusBadRequest)
	}

	// Test 2: Try to access URL with wrong password
	redirectReq, _ = http.NewRequest("GET", "/redirect?code="+shortCode, nil)
	redirectReq.Header.Set("X-Password", "wrongpass")
	redirectRR = httptest.NewRecorder()
	redirectHandler.ServeHTTP(redirectRR, redirectReq)

	if status := redirectRR.Code; status != http.StatusUnauthorized {
		t.Errorf("handler should return Unauthorized for wrong password: got %v want %v",
			status, http.StatusUnauthorized)
	}

	// Test 3: Access URL with correct password
	redirectReq, _ = http.NewRequest("GET", "/redirect?code="+shortCode, nil)
	redirectReq.Header.Set("X-Password", password)
	redirectRR = httptest.NewRecorder()
	redirectHandler.ServeHTTP(redirectRR, redirectReq)

	if status := redirectRR.Code; status != http.StatusTemporaryRedirect {
		t.Errorf("handler should return TemporaryRedirect for correct password: got %v want %v",
			status, http.StatusTemporaryRedirect)
	}

	if location := redirectRR.Header().Get("Location"); location != originalUrl {
		t.Errorf("redirect handler returned wrong location: got %v want %v",
			location, originalUrl)
	}

	// Clean up
	db.Unscoped().Delete(&UrlShortener{ShortCode: shortCode})
}

func TestGetUserUrlsRepoFunction(t *testing.T) {
	db := InitTest()
	ctx := context.Background()
	ctx = addValueToContext(&ctx, "db", db)

	urls := getUrlsByUserId(&ctx, 1, 1, 10)

	fmt.Println(len(urls))
	if len(urls) <= 0 {
		t.Errorf("Expected urls, got %d", len(urls))
	}
}

func TestGetUserUrls(t *testing.T) {
	db := InitTest()
	ctx := context.Background()
	ctx = addValueToContext(&ctx, "db", db)

	// Create a test user
	testUser := &Users{
		Email:     uuid.New().String()[:5] + "@example.com",
		Name:      addressOf("Test User"),
		ApiKey:    "test_api_key_urls_" + uuid.NewString(),
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	db.Create(testUser)

	testUser = getUserFromApiKeyIfExists(&ctx, testUser.ApiKey)
	ctx = addValueToContext(&ctx, "user", testUser)

	// Create multiple URLs for this user
	urls := []UrlShortener{
		{
			OriginalUrl: "http://example1.com",
			ShortCode:   uuid.New().String()[:5],
			UserId:      &testUser.Id,
			CreatedAt:   time.Now(),
			UpdatedAt:   time.Now(),
		},
		{
			OriginalUrl: "http://example2.com",
			ShortCode:   uuid.New().String()[:5],
			UserId:      &testUser.Id,
			CreatedAt:   time.Now(),
			UpdatedAt:   time.Now(),
		},
	}

	for _, url := range urls {
		db.Create(&url)
	}

	// Test 1: Get URLs with valid API key
	req, err := http.NewRequest("GET", "/urls", nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("X-API-Key", testUser.ApiKey)

	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(ctxServiceHandler(getUserUrls, &ctx))
	handler.ServeHTTP(rr, req)

	// Check status code
	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v",
			status, http.StatusOK)
	}

	// Parse response
	var response []UrlShortener
	if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
		t.Fatal("Failed to decode response body")
	}

	// Verify number of URLs returned
	if len(response) != len(urls) {
		t.Errorf("Expected %d URLs, got %d", len(urls), len(response))
	}

	// Verify URL contents
	urlMap := make(map[string]bool)
	for _, url := range response {
		urlMap[url.ShortCode] = true
		if url.UserId == nil || *url.UserId != testUser.Id {
			t.Errorf("URL %s not properly linked to user", url.ShortCode)
		}
	}
	for _, url := range urls {
		if !urlMap[url.ShortCode] {
			t.Errorf("Expected URL %s not found in response", url.ShortCode)
		}
	}

	ctx = context.Background()
	ctx = addValueToContext(&ctx, "db", db)

	// Test 2: Try without API key
	req, _ = http.NewRequest("GET", "/urls", nil)
	rr = httptest.NewRecorder()
	handler = http.HandlerFunc(ctxServiceHandler(getUserUrls, &ctx))
	handler.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusNotFound {
		t.Errorf("handler should return NotFound without API key: got %v want %v",
			status, http.StatusNotFound)
	}

	// Test 3: Try with invalid API key
	req, _ = http.NewRequest("GET", "/urls", nil)
	req.Header.Set("X-API-Key", "invalid_key")
	rr = httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusNotFound {
		t.Errorf("handler should return NotFound for invalid API key: got %v want %v",
			status, http.StatusNotFound)
	}

	// Clean up
	for _, url := range urls {
		db.Unscoped().Delete(&url)
	}
	db.Unscoped().Delete(testUser)
}

func TestRedirectCaching(t *testing.T) {
	db := InitTest()
	ctx := context.Background()
	ctx = addValueToContext(&ctx, "db", db)

	// Initialize Redis client for testing
	initRedis()

	// Create a test URL
	originalUrl := "http://example.com/caching-test"
	shortenReqBody := strings.NewReader(`{"url": "` + originalUrl + `"}`)
	shortenReq, err := http.NewRequest("POST", "/shorten", shortenReqBody)
	if err != nil {
		t.Fatal(err)
	}
	shortenReq.Header.Set("Content-Type", "application/json")

	shortenRR := httptest.NewRecorder()
	handler := http.HandlerFunc(ctxServiceHandler(shortenUrl, &ctx))
	handler.ServeHTTP(shortenRR, shortenReq)

	// Check if URL was created successfully
	if status := shortenRR.Code; status != http.StatusCreated {
		t.Errorf("handler returned wrong status code: got %v want %v", status, http.StatusCreated)
	}

	// Extract the shortCode from the response
	var response map[string]string
	if err := json.NewDecoder(shortenRR.Body).Decode(&response); err != nil {
		t.Fatal("Failed to decode response body")
	}
	shortCode := response["short_code"]

	// Verify Redis cache is empty before first request
	cachedUrl, err := getCachedUrl(shortCode)
	if err != nil {
		t.Fatalf("Error checking Redis cache: %v", err)
	}
	if cachedUrl != nil {
		t.Error("URL should not be in Redis cache before first request")
	}

	// First request - should hit the database and populate the Redis cache
	redirectReq1, _ := http.NewRequest("GET", "/redirect?code="+shortCode, nil)
	redirectRR1 := httptest.NewRecorder()
	redirectHandler := http.HandlerFunc(ctxServiceHandler(redirectToOriginalUrl, &ctx))
	redirectHandler.ServeHTTP(redirectRR1, redirectReq1)

	// Verify first request was successful
	if status := redirectRR1.Code; status != http.StatusTemporaryRedirect {
		t.Errorf("First request failed with status: %v", status)
	}

	// Verify URL was cached in Redis after first request
	cachedUrl, err = getCachedUrl(shortCode)
	if err != nil {
		t.Fatalf("Error checking Redis cache: %v", err)
	}
	if cachedUrl == nil {
		t.Error("URL was not cached in Redis after first request")
	}

	// Delete the URL from the database to ensure subsequent requests use the Redis cache
	var urlModel UrlShortener
	db.Unscoped().Where("short_code = ?", shortCode).Delete(&urlModel)

	// Second request - should use Redis cache since the DB record is gone
	redirectReq2, _ := http.NewRequest("GET", "/redirect?code="+shortCode, nil)
	redirectRR2 := httptest.NewRecorder()
	redirectHandler.ServeHTTP(redirectRR2, redirectReq2)

	// Verify second request was successful (should use Redis cache)
	if status := redirectRR2.Code; status != http.StatusTemporaryRedirect {
		t.Errorf("Second request failed with status: %v, expected %v (should use Redis cache)",
			status, http.StatusTemporaryRedirect)
	}

	// Verify both responses redirect to the same URL
	location1 := redirectRR1.Header().Get("Location")
	location2 := redirectRR2.Header().Get("Location")
	if location1 != location2 || location1 != originalUrl {
		t.Errorf("Redirect locations don't match. Got %v and %v, expected both to be %v",
			location1, location2, originalUrl)
	}

	// Verify that if we clear the Redis cache and try again, it fails (proving we were using the cache)
	err = removeCachedUrl(shortCode)
	if err != nil {
		t.Fatalf("Error clearing Redis cache: %v", err)
	}

	// Verify the cache is now empty
	cachedUrl, err = getCachedUrl(shortCode)
	if err != nil {
		t.Fatalf("Error checking Redis cache: %v", err)
	}
	if cachedUrl != nil {
		t.Error("URL should not be in Redis cache after clearing")
	}

	// Third request - should fail since DB record is gone and Redis cache is cleared
	redirectReq3, _ := http.NewRequest("GET", "/redirect?code="+shortCode, nil)
	redirectRR3 := httptest.NewRecorder()
	redirectHandler.ServeHTTP(redirectRR3, redirectReq3)

	// This should fail with 404 since the DB record is gone and Redis cache is cleared
	if status := redirectRR3.Code; status != http.StatusNotFound {
		t.Errorf("Third request should fail with NotFound after Redis cache cleared: got %v", status)
	}

	// Test updating the cache
	// Create a new URL in the database
	newUrlModel := &UrlShortener{
		OriginalUrl: "http://updated-example.com",
		ShortCode:   shortCode,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}
	db.Create(newUrlModel)

	// Cache the URL
	err = cacheUrl(shortCode, newUrlModel)
	if err != nil {
		t.Fatalf("Error caching URL: %v", err)
	}

	// Update the URL in the cache
	updatedUrlModel := &UrlShortener{
		OriginalUrl: "http://updated-again-example.com",
		ShortCode:   shortCode,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	// Update the cache
	err = updateCachedUrl(shortCode, updatedUrlModel)
	if err != nil {
		t.Fatalf("Error updating cached URL: %v", err)
	}

	// Verify the cache was updated
	cachedUrl, err = getCachedUrl(shortCode)
	if err != nil {
		t.Fatalf("Error checking Redis cache: %v", err)
	}
	if cachedUrl == nil {
		t.Error("URL should be in Redis cache after updating")
	}
	if cachedUrl.OriginalUrl != updatedUrlModel.OriginalUrl {
		t.Errorf("Expected updated URL %s, got %s", updatedUrlModel.OriginalUrl, cachedUrl.OriginalUrl)
	}

	// Clean up
	removeCachedUrl(shortCode)
	db.Unscoped().Delete(&UrlShortener{ShortCode: shortCode})
}

func TestIpRateLimitMiddleware(t *testing.T) {
	// Initialize Redis for testing
	initRedis()

	// Create a simple test handler that always returns 200 OK
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	// Apply the rate limit middleware to the test handler
	handler := ipRateLimitMiddleware()(testHandler)

	// Test cases for different endpoints
	testCases := []struct {
		name           string
		path           string
		requestCount   int
		rateLimit      int64
		expectedStatus int
	}{
		{
			name:           "Redirect endpoint under limit",
			path:           "/redirect",
			requestCount:   45,
			rateLimit:      50,
			expectedStatus: http.StatusOK,
		},
		{
			name:           "Redirect endpoint over limit",
			path:           "/redirect",
			requestCount:   55,
			rateLimit:      50,
			expectedStatus: http.StatusTooManyRequests,
		},
		{
			name:           "Shorten endpoint under limit",
			path:           "/shorten",
			requestCount:   8,
			rateLimit:      10,
			expectedStatus: http.StatusOK,
		},
		{
			name:           "Shorten endpoint over limit",
			path:           "/shorten",
			requestCount:   12,
			rateLimit:      10,
			expectedStatus: http.StatusTooManyRequests,
		},
		{
			name:           "Default endpoint under limit",
			path:           "/other",
			requestCount:   90,
			rateLimit:      100,
			expectedStatus: http.StatusOK,
		},
		{
			name:           "Default endpoint over limit",
			path:           "/other",
			requestCount:   110,
			rateLimit:      100,
			expectedStatus: http.StatusTooManyRequests,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Generate a unique IP for this test to avoid interference between test cases
			testIP := fmt.Sprintf("test-ip-%s-%d", tc.path, time.Now().UnixNano())

			// Clear any existing rate limit data for this test IP
			redisKey := ""
			if tc.path == "/redirect" {
				redisKey = "redirect:" + testIP
			} else if tc.path == "/shorten" {
				redisKey = "shorten:" + testIP
			} else {
				redisKey = "default:" + testIP
			}
			redisClient.Del(redisKey)

			// Make requests up to the specified count
			var lastStatus int
			for i := 1; i <= tc.requestCount; i++ {
				req, err := http.NewRequest("GET", tc.path, nil)
				if err != nil {
					t.Fatalf("Failed to create request: %v", err)
				}

				// Set the remote address to our test IP
				req.RemoteAddr = testIP

				rr := httptest.NewRecorder()
				handler.ServeHTTP(rr, req)
				lastStatus = rr.Code

				// Add a small delay to avoid overwhelming Redis
				time.Sleep(1 * time.Millisecond)
			}

			// Check if the last request had the expected status code
			if lastStatus != tc.expectedStatus {
				t.Errorf("Expected status %d after %d requests, got %d",
					tc.expectedStatus, tc.requestCount, lastStatus)
			}

			// If we're testing over the limit, also verify that after waiting for the rate limit window
			// to expire, we can make requests again
			if tc.expectedStatus == http.StatusTooManyRequests {
				// Wait for the rate limit window to expire (slightly longer than the window)
				if tc.path == "/redirect" || tc.path == "/shorten" {
					time.Sleep(1 * time.Second) // 1.1 seconds for 1 second window
				} else {
					time.Sleep(1 * time.Minute) // We'll use a shorter wait for testing
				}

				// Try one more request, which should now succeed
				req, _ := http.NewRequest("GET", tc.path, nil)
				req.RemoteAddr = testIP
				rr := httptest.NewRecorder()
				handler.ServeHTTP(rr, req)

				if rr.Code != http.StatusOK {
					t.Errorf("Expected status %d after rate limit window expired, got %d",
						http.StatusOK, rr.Code)
				}
			}

			// Clean up
			redisClient.Del(redisKey)
		})
	}
}

func TestCreateNUrlEntriesBatch(t *testing.T) {
	db := InitTest()
	ctx := context.Background()
	initRedis()
	ctx = addValueToContext(&ctx, "db", db)

	n := 15_00_000   // Number of entries
	batchSize := 100 // Insert in batches of 100

	startTime := time.Now()

	for i := 0; i < n; i += batchSize {
		batch := make([]UrlShortener, 0, batchSize)

		// Create a batch of entries
		end := min(i+batchSize, n)

		for j := i; j < end; j++ {
			originalUrl := fmt.Sprintf("https://example.com/test/%d/%s", j, uuid.New().String())

			batch = append(batch, UrlShortener{
				OriginalUrl: originalUrl,
				ShortCode:   createShortCode(&ctx, 0),
				CreatedAt:   time.Now(),
				UpdatedAt:   time.Now(),
			})
		}

		// Insert the batch
		result := db.Create(&batch)
		if result.Error != nil {
			t.Fatalf("Failed to insert batch starting at %d: %v", i, result.Error)
		}

		t.Logf("Created entries %d to %d", i, end-1)
	}

	elapsed := time.Since(startTime)
	t.Logf("Created %d URL entries in %v (%.2f entries/sec)",
		n, elapsed, float64(n)/elapsed.Seconds())
}
