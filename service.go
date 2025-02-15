package main

import (
	"context"
	"encoding/json"
	"net/http"
)

func health(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

func shortenUrl(ctx *context.Context, w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var requestBody struct {
		URL string `json:"url"`
	}

	if err := json.NewDecoder(r.Body).Decode(&requestBody); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if requestBody.URL == "" {
		http.Error(w, "URL is required", http.StatusBadRequest)
		return
	}

	if doesUrlExist(ctx, requestBody.URL) {
		shortCode := getShortCode(ctx, requestBody.URL)
		w.WriteHeader(http.StatusCreated)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"short_code": shortCode})
		return
	}

	shortCode := createShortCode(ctx, requestBody.URL)

	insertUrl(ctx, requestBody.URL, shortCode)

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

	if !doesShortCodeExist(ctx, shortCode) {
		http.Error(w, "Short code not found", http.StatusNotFound)
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
