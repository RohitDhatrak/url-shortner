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

// func TestUrlActivationAndDeactivation(t *testing.T) {
// 	db := InitTest()
// 	ctx := context.Background()
// 	ctx = addValueToContext(&ctx, "db", db)

// 	// Create a test user
// 	testUser := &Users{
// 		Email:     "test-activation@example.com",
// 		Name:      addressOf("Test Activation User"),
// 		ApiKey:    "test_api_key_activation",
// 		CreatedAt: time.Now(),
// 		UpdatedAt: time.Now(),
// 	}
// 	db.Create(testUser)

// 	// Add user to context
// 	testUser = getUserFromApiKeyIfExists(&ctx, testUser.ApiKey)
// 	ctx = addValueToContext(&ctx, "user", testUser)

// 	// Create a URL with the test user
// 	originalUrl := "http://example.com/activation-test"
// 	shortenReqBody := strings.NewReader(fmt.Sprintf(`{
// 		"url": "%s"
// 	}`, originalUrl))

// 	shortenReq, err := http.NewRequest("POST", "/shorten", shortenReqBody)
// 	if err != nil {
// 		t.Fatal(err)
// 	}
// 	shortenReq.Header.Set("Content-Type", "application/json")
// 	shortenReq.Header.Set("X-API-Key", testUser.ApiKey)

// 	shortenRR := httptest.NewRecorder()
// 	handler := http.HandlerFunc(ctxServiceHandler(shortenUrl, &ctx))
// 	handler.ServeHTTP(shortenRR, shortenReq)

// 	// Check if URL was created successfully
// 	if status := shortenRR.Code; status != http.StatusCreated {
// 		t.Errorf("handler returned wrong status code: got %v want %v",
// 			status, http.StatusCreated)
// 	}

// 	// Get the short code from response
// 	var response map[string]string
// 	if err := json.NewDecoder(shortenRR.Body).Decode(&response); err != nil {
// 		t.Fatal("Failed to decode response body")
// 	}
// 	shortCode := response["short_code"]
// 	if shortCode == "" {
// 		t.Fatal("No short code returned in response")
// 	}

// 	// Verify URL exists and is active
// 	urlModel := getUrlModel(&ctx, shortCode)
// 	if urlModel == nil {
// 		t.Fatal("URL should exist after creation")
// 	}
// 	if urlModel.DeletedAt != nil {
// 		t.Fatal("URL should not be marked as deleted after creation")
// 	}

// 	// Test 1: Deactivate URL
// 	deactivateBody := strings.NewReader(fmt.Sprintf(`{
// 		"short_code": "%s",
// 		"activate": false
// 	}`, shortCode))

// 	deactivateReq, err := http.NewRequest("PUT", "/url", deactivateBody)
// 	if err != nil {
// 		t.Fatal(err)
// 	}
// 	deactivateReq.Header.Set("Content-Type", "application/json")
// 	deactivateReq.Header.Set("X-API-Key", testUser.ApiKey)

// 	deactivateRR := httptest.NewRecorder()
// 	editHandler := http.HandlerFunc(ctxServiceHandler(editUrl, &ctx))
// 	editHandler.ServeHTTP(deactivateRR, deactivateReq)

// 	// Check deactivation response
// 	if status := deactivateRR.Code; status != http.StatusOK {
// 		t.Errorf("handler returned wrong status code for deactivation: got %v want %v",
// 			status, http.StatusOK)
// 	}

// 	// Verify URL is deactivated
// 	urlModel = getUrlModel(&ctx, shortCode)
// 	if urlModel != nil {
// 		t.Error("URL should not be found after deactivation")
// 	}

// 	// Get the URL directly from the database to check DeletedAt
// 	var deactivatedUrl UrlShortener
// 	result := db.Where("short_code = ?", shortCode).First(&deactivatedUrl)
// 	if result.Error != nil {
// 		t.Fatal("Failed to fetch URL:", result.Error)
// 	}
// 	if deactivatedUrl.DeletedAt == nil {
// 		t.Error("URL should be marked as deleted after deactivation")
// 	}

// 	// Test 2: Activate URL
// 	activateBody := strings.NewReader(fmt.Sprintf(`{
// 		"short_code": "%s",
// 		"activate": true
// 	}`, shortCode))

// 	activateReq, err := http.NewRequest("PUT", "/url", activateBody)
// 	if err != nil {
// 		t.Fatal(err)
// 	}
// 	activateReq.Header.Set("Content-Type", "application/json")
// 	activateReq.Header.Set("X-API-Key", testUser.ApiKey)

// 	activateRR := httptest.NewRecorder()
// 	editHandler.ServeHTTP(activateRR, activateReq)

// 	// Check activation response
// 	if status := activateRR.Code; status != http.StatusOK {
// 		t.Errorf("handler returned wrong status code for activation: got %v want %v",
// 			status, http.StatusOK)
// 	}

// 	// Verify URL is activated
// 	urlModel = getUrlModel(&ctx, shortCode)
// 	if urlModel == nil {
// 		t.Error("URL should be found after activation")
// 	}

// 	// Get the URL directly from the database to check DeletedAt
// 	var activatedUrl UrlShortener
// 	result = db.Where("short_code = ?", shortCode).First(&activatedUrl)
// 	if result.Error != nil {
// 		t.Fatal("Failed to fetch URL:", result.Error)
// 	}
// 	if activatedUrl.DeletedAt != nil {
// 		t.Error("URL should not be marked as deleted after activation")
// 	}

// 	// Test 3: Try to edit URL with a different user
// 	// Create another test user
// 	anotherUser := &Users{
// 		Email:     "another-user@example.com",
// 		Name:      addressOf("Another User"),
// 		ApiKey:    "another_user_api_key",
// 		CreatedAt: time.Now(),
// 		UpdatedAt: time.Now(),
// 	}
// 	db.Create(anotherUser)

// 	// Add the other user to context
// 	anotherUser = getUserFromApiKeyIfExists(&ctx, anotherUser.ApiKey)
// 	ctx = addValueToContext(&ctx, "user", anotherUser)

// 	// Try to deactivate URL with another user
// 	deactivateReq, _ = http.NewRequest("PUT", "/url", deactivateBody)
// 	deactivateReq.Header.Set("Content-Type", "application/json")
// 	deactivateReq.Header.Set("X-API-Key", anotherUser.ApiKey)

// 	deactivateRR = httptest.NewRecorder()
// 	editHandler.ServeHTTP(deactivateRR, deactivateReq)

// 	// Should get Forbidden status
// 	if status := deactivateRR.Code; status != http.StatusForbidden {
// 		t.Errorf("handler should return Forbidden for unauthorized user: got %v want %v",
// 			status, http.StatusForbidden)
// 	}

// 	// Clean up
// 	db.Unscoped().Delete(&UrlShortener{ShortCode: shortCode})
// 	db.Unscoped().Delete(testUser)
// 	db.Unscoped().Delete(anotherUser)
// }

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

	urls := getUrlsByUserId(&ctx, 1)

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

// oha -c 100 -q 100 -z 20s -m POST http://localhost:8080/redirect?code=32d1t
