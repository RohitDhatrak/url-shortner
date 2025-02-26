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
	ctx = AddValueToContext(&ctx, "db", db)

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
	handler := http.HandlerFunc(CtxServiceHandler(shortenUrl, &ctx))

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
	redirectHandler := http.HandlerFunc(CtxServiceHandler(redirectToOriginalUrl, &ctx))

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
	ctx = AddValueToContext(&ctx, "db", db)

	// Simulate a GET request to the redirect endpoint with a non-existent short code
	nonExistentShortCode := "nonexistent123"
	redirectReq, err := http.NewRequest("GET", "/redirect?code="+nonExistentShortCode, nil)
	if err != nil {
		t.Fatal(err)
	}

	// Create a new ResponseRecorder for the redirect request
	redirectRR := httptest.NewRecorder()
	redirectHandler := http.HandlerFunc(CtxServiceHandler(redirectToOriginalUrl, &ctx))

	// Serve the redirect request
	redirectHandler.ServeHTTP(redirectRR, redirectReq)

	// Check if the response status code is 404
	if status := redirectRR.Code; status != http.StatusNotFound {
		t.Errorf("handler returned wrong status code: got %v want %v", status, http.StatusNotFound)
	}
}

func TestDeleteShortCode(t *testing.T) {
	db := InitTest()

	ctx := context.Background()
	ctx = AddValueToContext(&ctx, "db", db)

	// First create a URL to get a short code
	originalUrl := "http://example.com"
	shortenReqBody := strings.NewReader(`{"url": "` + originalUrl + `"}`)
	shortenReq, err := http.NewRequest("POST", "/shorten", shortenReqBody)
	if err != nil {
		t.Fatal(err)
	}
	shortenReq.Header.Set("Content-Type", "application/json")

	shortenRR := httptest.NewRecorder()
	handler := http.HandlerFunc(CtxServiceHandler(shortenUrl, &ctx))
	handler.ServeHTTP(shortenRR, shortenReq)

	// Extract the shortCode from the response
	var response map[string]string
	if err := json.NewDecoder(shortenRR.Body).Decode(&response); err != nil {
		t.Fatal("Failed to decode response body")
	}
	shortCode := response["short_code"]

	// Now test the delete endpoint
	deleteReq, err := http.NewRequest("DELETE", "/shorten?code="+shortCode, nil)
	if err != nil {
		t.Fatal(err)
	}

	deleteRR := httptest.NewRecorder()
	deleteHandler := http.HandlerFunc(CtxServiceHandler(shortenUrlHandler, &ctx))
	deleteHandler.ServeHTTP(deleteRR, deleteReq)

	// Check if deletion was successful
	fmt.Println("LOG: ", deleteRR.Body.String())
	if status := deleteRR.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v",
			status, http.StatusOK)
	}

	// Verify the short code no longer exists by trying to redirect to it
	redirectReq, err := http.NewRequest("GET", "/redirect?code="+shortCode, nil)
	if err != nil {
		t.Fatal(err)
	}

	redirectRR := httptest.NewRecorder()
	redirectHandler := http.HandlerFunc(CtxServiceHandler(redirectToOriginalUrl, &ctx))
	redirectHandler.ServeHTTP(redirectRR, redirectReq)

	// Should get a 404 since the short code was deleted
	if status := redirectRR.Code; status != http.StatusNotFound {
		t.Errorf("handler returned wrong status code: got %v want %v",
			status, http.StatusNotFound)
	}
}

func TestShortenEmptyUrl(t *testing.T) {
	db := InitTest()

	ctx := context.Background()
	ctx = AddValueToContext(&ctx, "db", db)

	// Simulate a POST request with an empty URL in the request body
	shortenReqBody := strings.NewReader(`{"url": ""}`)
	shortenReq, err := http.NewRequest("POST", "/shorten", shortenReqBody)
	if err != nil {
		t.Fatal(err)
	}
	shortenReq.Header.Set("Content-Type", "application/json")

	// Create a ResponseRecorder to record the response
	shortenRR := httptest.NewRecorder()
	handler := http.HandlerFunc(CtxServiceHandler(shortenUrl, &ctx))

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
	ctx = AddValueToContext(&ctx, "db", db)

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
		handler := http.HandlerFunc(CtxServiceHandler(shortenUrl, &ctx))
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
	ctx = AddValueToContext(&ctx, "db", db)

	// First create a test user
	testUser := &Users{
		Email:     "test@example.com",
		Name:      "Test User",
		ApiKey:    "test_api_key_123",
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
	handler := http.HandlerFunc(CtxServiceHandler(shortenUrl, &ctx))
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
	ctx = AddValueToContext(&ctx, "db", db)

	// Create two test users
	user1 := &Users{
		Email:     "test-user1@example.com",
		Name:      "Test User One",
		ApiKey:    "test_api_key_user1",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	user2 := &Users{
		Email:     "test-user2@example.com",
		Name:      "Test User Two",
		ApiKey:    "test_api_key_user2",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	db.Create(user1)
	db.Create(user2)

	// Create a URL with user1's API key
	originalUrl := "http://example.com"
	shortenReqBody := strings.NewReader(`{"url": "` + originalUrl + `"}`)
	shortenReq, err := http.NewRequest("POST", "/shorten", shortenReqBody)
	if err != nil {
		t.Fatal(err)
	}
	shortenReq.Header.Set("Content-Type", "application/json")
	shortenReq.Header.Set("X-API-Key", user1.ApiKey)

	shortenRR := httptest.NewRecorder()
	handler := http.HandlerFunc(CtxServiceHandler(shortenUrl, &ctx))
	handler.ServeHTTP(shortenRR, shortenReq)

	var response map[string]string
	if err := json.NewDecoder(shortenRR.Body).Decode(&response); err != nil {
		t.Fatal("Failed to decode response body")
	}
	shortCode := response["short_code"]

	// Test 1: Try to delete with user2's API key (should fail)
	deleteReq, _ := http.NewRequest("DELETE", "/shorten?code="+shortCode, nil)
	deleteReq.Header.Set("X-API-Key", user2.ApiKey)
	deleteRR := httptest.NewRecorder()
	deleteHandler := http.HandlerFunc(CtxServiceHandler(shortenUrlHandler, &ctx))
	deleteHandler.ServeHTTP(deleteRR, deleteReq)

	if status := deleteRR.Code; status != http.StatusForbidden {
		t.Errorf("Expected status forbidden for unauthorized user, got %v", status)
	}

	// Test 2: Delete with user1's API key (should succeed)
	deleteReq, _ = http.NewRequest("DELETE", "/shorten?code="+shortCode, nil)
	deleteReq.Header.Set("X-API-Key", user1.ApiKey)
	deleteRR = httptest.NewRecorder()
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
	ctx = AddValueToContext(&ctx, "db", db)

	shortCode := "2wk9m"
	exists := doesShortCodeExist(&ctx, shortCode)

	if exists {
		t.Errorf("Expected short code to not exist, got %v", exists)
	}

	originalUrl := getOriginalUrl(&ctx, shortCode)

	if originalUrl != "" {
		t.Errorf("Expected original URL to be empty, got %v", originalUrl)
	}

	urlModel := getUrlModel(&ctx, shortCode)

	if urlModel != nil {
		t.Errorf("Expected URL model to be nil, got %v", urlModel)
	}
}

func TestUrlExpiration(t *testing.T) {
	db := InitTest()
	ctx := context.Background()
	ctx = AddValueToContext(&ctx, "db", db)

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
	handler := http.HandlerFunc(CtxServiceHandler(shortenUrl, &ctx))
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
	redirectHandler := http.HandlerFunc(CtxServiceHandler(redirectToOriginalUrl, &ctx))
	redirectHandler.ServeHTTP(redirectRR, redirectReq)

	if status := redirectRR.Code; status != http.StatusNotFound {
		t.Errorf("Expected status not found for expired URL, got %v", status)
	}
}
