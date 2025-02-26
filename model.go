package main

import "time"

const MAX_RETRIES = 3

type UrlShortener struct {
	OriginalUrl string    `gorm:"not null"`
	ShortCode   string    `gorm:"not null"`
	Views       int       `gorm:"default:0"`
	LastViewed  time.Time `gorm:"default:null"`
	UserId      *uint     `gorm:"default:null;foreignKey:Id;references:Users"`
	User        Users     `gorm:"foreignKey:UserId"`
	CreatedAt   time.Time `gorm:"not null"`
	UpdatedAt   time.Time `gorm:"not null"`
	DeletedAt   time.Time `gorm:"default:null"`
	ExpiresAt   time.Time `gorm:"default:null"`
}

type Users struct {
	Id        uint      `gorm:"primaryKey"`
	Email     string    `gorm:"unique;not null"`
	Name      string    `gorm:"default:null"`
	ApiKey    string    `gorm:"unique;not null"`
	CreatedAt time.Time `gorm:"not null"`
	UpdatedAt time.Time `gorm:"not null"`
	DeletedAt time.Time `gorm:"default:null"`
}
