package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

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

func TestSameUrlReturnsSameShortCode(t *testing.T) {
	db := InitTest()

	ctx := context.Background()
	ctx = AddValueToContext(&ctx, "db", db)

	originalUrl := "http://example.com"
	shortenReqBody := strings.NewReader(`{"url": "` + originalUrl + `"}`)

	// First request
	shortenReq1, err := http.NewRequest("POST", "/shorten", shortenReqBody)
	if err != nil {
		t.Fatal(err)
	}
	shortenReq1.Header.Set("Content-Type", "application/json")

	shortenRR1 := httptest.NewRecorder()
	handler := http.HandlerFunc(CtxServiceHandler(shortenUrl, &ctx))
	handler.ServeHTTP(shortenRR1, shortenReq1)

	if status := shortenRR1.Code; status != http.StatusCreated {
		t.Errorf("handler returned wrong status code: got %v want %v", status, http.StatusCreated)
	}

	var response1 map[string]string
	if err := json.NewDecoder(shortenRR1.Body).Decode(&response1); err != nil {
		t.Fatal("Failed to decode response body")
	}
	shortCode1, exists := response1["short_code"]
	if !exists {
		t.Fatal("short_code not found in response")
	}

	// Second request
	shortenReq2, err := http.NewRequest("POST", "/shorten", strings.NewReader(`{"url": "`+originalUrl+`"}`))
	if err != nil {
		t.Fatal(err)
	}
	shortenReq2.Header.Set("Content-Type", "application/json")

	shortenRR2 := httptest.NewRecorder()
	handler.ServeHTTP(shortenRR2, shortenReq2)

	if status := shortenRR2.Code; status != http.StatusCreated {
		t.Errorf("handler returned wrong status code: got %v want %v", status, http.StatusCreated)
	}

	var response2 map[string]string
	if err := json.NewDecoder(shortenRR2.Body).Decode(&response2); err != nil {
		t.Fatal("Failed to decode response body")
	}
	shortCode2, exists := response2["short_code"]
	if !exists {
		t.Fatal("short_code not found in response")
	}

	// Check if both short codes are the same
	if shortCode1 != shortCode2 {
		t.Errorf("Expected the same short code for the same URL, got %v and %v", shortCode1, shortCode2)
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

	// Simulate a POST request to create a new short URL
	originalUrl := "http://example.com/delete-test"
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

	// Simulate a DELETE request to delete the short URL
	deleteReq, err := http.NewRequest("DELETE", "/shorten?code="+shortCode, nil)
	if err != nil {
		t.Fatal(err)
	}

	// Create a new ResponseRecorder for the delete request
	deleteRR := httptest.NewRecorder()
	deleteHandler := http.HandlerFunc(CtxServiceHandler(deleteShortCode, &ctx))

	// Serve the delete request
	deleteHandler.ServeHTTP(deleteRR, deleteReq)

	// Check the status code
	if status := deleteRR.Code; status != http.StatusOK {
		t.Errorf("delete handler returned wrong status code: got %v want %v", status, http.StatusOK)
	}

	// Verify that the short code no longer exists
	if doesShortCodeExist(&ctx, shortCode) {
		t.Errorf("short code %v still exists after deletion", shortCode)
	}
}
