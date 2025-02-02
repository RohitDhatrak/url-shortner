package main

import (
	"time"
)

type UrlShortener struct {
	OriginalUrl string    `gorm:"not null"`
	ShortCode   string    `gorm:"unique;not null"`
	CreatedAt   time.Time `gorm:"not null"`
	UpdatedAt   time.Time `gorm:"not null"`
	DeletedAt   time.Time `gorm:"default:null"`
}

type UrlShortenerMongoDb struct {
	OriginalUrl string    `bson:"original_url"`
	ShortCode   string    `bson:"short_code"`
	CreatedAt   time.Time `bson:"created_at"`
	UpdatedAt   time.Time `bson:"updated_at"`
}
