package main

import "time"

const MAX_RETRIES = 3
const NORMAL_SHORT_CODE_LENGTH = 8

type UrlShortener struct {
	OriginalUrl string    `gorm:"not null"`
	ShortCode   string    `gorm:"unique;not null"`
	CreatedAt   time.Time `gorm:"not null"`
	UpdatedAt   time.Time `gorm:"not null"`
	DeletedAt   time.Time `gorm:"default:null"`
}
