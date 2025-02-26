package main

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
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
		CustomUrl *string `json:"custom_url"`
	}

	if err := json.NewDecoder(r.Body).Decode(&requestBody); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if requestBody.URL == "" {
		http.Error(w, "URL is required", http.StatusBadRequest)
		return
	}

	if requestBody.CustomUrl != nil && *requestBody.CustomUrl == "" {
		http.Error(w, "Custom URL cannot be empty", http.StatusBadRequest)
		return
	}

	apiKey := r.Header.Get("X-API-Key")
	user := getUserFromApiKeyIfExists(ctx, apiKey)

	shortCode := ""
	if requestBody.CustomUrl != nil {
		if doesShortCodeExist(ctx, *requestBody.CustomUrl) {
			http.Error(w, "This custom URL already exists", http.StatusBadRequest)
			return
		}
		shortCode = *requestBody.CustomUrl
	} else {
		shortCode = createShortCode(ctx, 0)
	}

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

	err := insertUrl(ctx, urlShortner)
	if err != nil {
		http.Error(w, "Error creating the short URL", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusCreated)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"short_code": shortCode})
}

func shortenUrlBulk(ctx *context.Context, w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var requestBody struct {
		URLs []struct {
			URL       string  `json:"url"`
			ExpiresAt *string `json:"expires_at"`
			CustomUrl *string `json:"custom_url"`
		} `json:"urls"`
	}

	if err := json.NewDecoder(r.Body).Decode(&requestBody); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Validations
	if len(requestBody.URLs) == 0 {
		http.Error(w, "URLs are required", http.StatusBadRequest)
		return
	}

	existingCustomUrls := []string{}
	for i, urlStruct := range requestBody.URLs {
		if urlStruct.URL == "" {
			http.Error(w, "Empty url at position "+strconv.Itoa(i+1), http.StatusBadRequest)
			return
		}

		if urlStruct.CustomUrl != nil && *urlStruct.CustomUrl == "" {
			http.Error(w, "Custom URL cannot be empty", http.StatusBadRequest)
			return
		}

		if urlStruct.CustomUrl != nil && doesShortCodeExist(ctx, *urlStruct.CustomUrl) {
			existingCustomUrls = append(existingCustomUrls, *urlStruct.CustomUrl)
		}
	}

	if len(existingCustomUrls) > 0 {
		w.WriteHeader(http.StatusBadRequest)
		w.Header().Set("Content-Type", "application/json")
		marshelledExistingCustomUrls, err := json.Marshal(existingCustomUrls)
		if err != nil {
			http.Error(w, "Something went wrong", http.StatusInternalServerError)
			return
		}

		json.NewEncoder(w).
			Encode(map[string]string{
				"existingCustomUrls": string(marshelledExistingCustomUrls),
				"message":            "These custom URLs already exist",
			})
		return
	}

	apiKey := r.Header.Get("X-API-Key")
	user := getUserFromApiKeyIfExists(ctx, apiKey)

	shortCodes := []string{}
	for _, urlStruct := range requestBody.URLs {
		shortCode := ""
		if urlStruct.CustomUrl != nil {
			shortCode = *urlStruct.CustomUrl
		} else {
			shortCode = createShortCode(ctx, 0)
		}

		urlShortener := &UrlShortener{OriginalUrl: urlStruct.URL, ShortCode: shortCode}

		if user != nil {
			urlShortener.UserId = &user.Id
		}

		if urlStruct.ExpiresAt != nil {
			expiresAt, err := time.Parse(time.RFC3339, *urlStruct.ExpiresAt)
			if err != nil {
				shortCodes = append(shortCodes, "Invalid expiry date")
				continue
			}
			urlShortener.ExpiresAt = expiresAt
		}

		err := insertUrl(ctx, urlShortener)
		if err == nil {
			shortCodes = append(shortCodes, shortCode)
		} else {
			shortCodes = append(shortCodes, "Error creating short URL")
		}
	}

	masheledShortCodes, err := json.Marshal(shortCodes)
	if err != nil {
		http.Error(w, "Something went wrong", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusCreated)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"short_codes": string(masheledShortCodes)})
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
