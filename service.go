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

	query := `
	INSERT INTO url_shorteners (original_url, short_code, created_at, updated_at)
	VALUES (?, ?, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
`

	db := GetDbFromContext(ctx)
	_, err := db.Exec(query, requestBody.URL, shortCode)
	if err != nil {
		panic(err)
	}

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
	http.Redirect(w, r, originalUrl, http.StatusTemporaryRedirect)
}
