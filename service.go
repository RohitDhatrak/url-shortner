package main

import (
	"context"
	"encoding/json"
	"net/http"
	"time"
)

func health(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

func shortenUrlHandler(ctx *context.Context, w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodPost:
		shortenUrl(ctx, w, r)
	case http.MethodDelete:
		deleteShortCode(ctx, w, r)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func shortenUrl(ctx *context.Context, w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var requestBody struct {
		URL       string  `json:"url"`
		ExpiresAt *string `json:"expires_at"`
	}

	if err := json.NewDecoder(r.Body).Decode(&requestBody); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if requestBody.URL == "" {
		http.Error(w, "URL is required", http.StatusBadRequest)
		return
	}

	apiKey := r.Header.Get("X-API-Key")
	user := getUserFromApiKeyIfExists(ctx, apiKey)

	shortCode := createShortCode(ctx, 0)

	urlShortner := &UrlShortener{OriginalUrl: requestBody.URL, ShortCode: shortCode}

	if user != nil {
		urlShortner.UserId = &user.Id
	}

	if requestBody.ExpiresAt != nil {
		expiresAt, err := time.Parse(time.RFC3339, *requestBody.ExpiresAt)
		if err != nil {
			http.Error(w, "Invalid expiry date", http.StatusBadRequest)
			return
		}
		urlShortner.ExpiresAt = expiresAt
	}

	insertUrl(ctx, urlShortner)

	w.WriteHeader(http.StatusCreated)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"short_code": shortCode})
}

func redirectToOriginalUrl(ctx *context.Context, w http.ResponseWriter, r *http.Request) {
	shortCode := r.URL.Query().Get("code")
	if shortCode == "" {
		http.Error(w, "Missing code parameter", http.StatusBadRequest)
		return
	}

	originalUrl := getOriginalUrl(ctx, shortCode)
	if originalUrl == "" {
		http.Error(w, "Short code not found", http.StatusNotFound)
		return
	}

	http.Redirect(w, r, originalUrl, http.StatusTemporaryRedirect)
}

func deleteShortCode(ctx *context.Context, w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	shortCode := r.URL.Query().Get("code")
	if shortCode == "" {
		http.Error(w, "Missing code parameter", http.StatusBadRequest)
		return
	}

	urlModel := getUrlModel(ctx, shortCode)
	if urlModel == nil {
		http.Error(w, "Short code not found", http.StatusNotFound)
		return
	}

	apiKey := r.Header.Get("X-API-Key")
	user := getUserFromApiKeyIfExists(ctx, apiKey)

	if urlModel.UserId == nil || *urlModel.UserId != user.Id {
		http.Error(w, "You are not authorized to delete this short code", http.StatusForbidden)
		return
	}

	if err := deleteUrl(ctx, shortCode); err != nil {
		http.Error(w, "Error deleting short code", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}
