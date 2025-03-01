package main

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"golang.org/x/crypto/bcrypt"
)

func health(ctx *context.Context, w http.ResponseWriter, r *http.Request) {
	db := getDbFromContext(ctx)

	result := db.Raw("SELECT 1")
	if result.Error != nil {
		w.WriteHeader(http.StatusServiceUnavailable)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{
			"status":  "error",
			"message": "Database connection failed",
		})
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

func shortenUrl(ctx *context.Context, w http.ResponseWriter, r *http.Request) {
	var requestBody struct {
		URL       string  `json:"url"`
		ExpiresAt *string `json:"expires_at"`
		CustomUrl *string `json:"custom_url"`
		Password  *string `json:"password"`
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

	urlShortener := &UrlShortener{OriginalUrl: requestBody.URL, ShortCode: shortCode}

	if user != nil {
		urlShortener.UserId = &user.Id
	}

	if requestBody.ExpiresAt != nil {
		expiresAt, err := time.Parse(time.RFC3339, *requestBody.ExpiresAt)
		if err != nil {
			http.Error(w, "Invalid expiry date", http.StatusBadRequest)
			return
		}
		urlShortener.ExpiresAt = &expiresAt
	}

	if requestBody.Password != nil {
		hashedPassword, err := bcrypt.GenerateFromPassword([]byte(*requestBody.Password), bcrypt.DefaultCost)
		if err != nil {
			http.Error(w, "Error creating the short URL", http.StatusInternalServerError)
			return
		}
		hashedPasswordString := string(hashedPassword)
		urlShortener.Password = &hashedPasswordString
	}

	err := insertUrl(ctx, urlShortener)
	if err != nil {
		http.Error(w, "Error creating the short URL", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusCreated)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"short_code": shortCode})
}

func shortenUrlBulk(ctx *context.Context, w http.ResponseWriter, r *http.Request) {
	user := getUserFromContext(ctx)

	var requestBody struct {
		URLs []struct {
			URL       string  `json:"url"`
			ExpiresAt *string `json:"expires_at"`
			CustomUrl *string `json:"custom_url"`
			Password  *string `json:"password"`
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
			urlShortener.ExpiresAt = &expiresAt
		}

		if urlStruct.Password != nil {
			hashedPassword, err := bcrypt.GenerateFromPassword([]byte(*urlStruct.Password), bcrypt.DefaultCost)
			if err != nil {
				http.Error(w, "Error creating the short URL", http.StatusInternalServerError)
				return
			}
			hashedPasswordString := string(hashedPassword)
			urlShortener.Password = &hashedPasswordString
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

func editUrl(ctx *context.Context, w http.ResponseWriter, r *http.Request) {
	var requestBody struct {
		ShortCode string `json:"short_code"`
		Activate  *bool  `json:"activate"`
	}

	if err := json.NewDecoder(r.Body).Decode(&requestBody); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	urlModel := getUrlModel(ctx, requestBody.ShortCode)
	if urlModel == nil {
		http.Error(w, "Short code not found", http.StatusNotFound)
		return
	}

	user := getUserFromContext(ctx)

	if urlModel.UserId == nil || *urlModel.UserId != user.Id {
		http.Error(w, "You are not authorized to delete this short code", http.StatusForbidden)
		return
	}

	if requestBody.ShortCode == "" {
		http.Error(w, "Short code is required", http.StatusBadRequest)
		return
	}

	if requestBody.Activate != nil {
		if *requestBody.Activate {
			activateUrl(ctx, requestBody.ShortCode)
		} else {
			deleteUrl(ctx, requestBody.ShortCode)
		}
	}

	w.WriteHeader(http.StatusOK)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

func redirectToOriginalUrl(ctx *context.Context, w http.ResponseWriter, r *http.Request) {
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

	if urlModel.Password != nil {
		password := r.Header.Get("X-Password")
		if password == "" {
			http.Error(w, "Password is required", http.StatusBadRequest)
			return
		}

		err := bcrypt.CompareHashAndPassword([]byte(*urlModel.Password), []byte(password))
		if err != nil {
			http.Error(w, "Invalid password", http.StatusUnauthorized)
			return
		}
	}

	http.Redirect(w, r, urlModel.OriginalUrl, http.StatusTemporaryRedirect)
}

func deleteShortCode(ctx *context.Context, w http.ResponseWriter, r *http.Request) {
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

	user := getUserFromContext(ctx)

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

func getUserUrls(ctx *context.Context, w http.ResponseWriter, r *http.Request) {
	user := getUserFromContext(ctx)

	if user == nil {
		http.Error(w, "User not found", http.StatusNotFound)
		return
	}

	urls := getUrlsByUserId(ctx, user.Id)

	w.WriteHeader(http.StatusOK)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(urls)
}
