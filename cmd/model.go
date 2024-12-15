package main

import (
	"gorm.io/gorm"
)

type UrlShortener struct {
	gorm.Model
	OriginalUrl string `gorm:"not null"`
	ShortCode   string `gorm:"unique;not null"`
}
