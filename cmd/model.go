package main

import (
	"time"

	"gorm.io/gorm"
)

type UrlShortener struct {
	gorm.Model
	OriginalUrl string `gorm:"not null"`
	ShortCode   string `gorm:"unique;not null"`
}

type UrlShortenerMongoDb struct {
	OriginalUrl string    `bson:"original_url"`
	ShortCode   string    `bson:"short_code"`
	CreatedAt   time.Time `bson:"created_at"`
	UpdatedAt   time.Time `bson:"updated_at"`
}
