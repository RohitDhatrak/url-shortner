package main

import "time"

const MAX_RETRIES = 3

type UrlShortener struct {
	OriginalUrl string     `gorm:"not null"`
	ShortCode   string     `gorm:"unique;not null"`
	Views       int        `gorm:"default:0"`
	LastViewed  *time.Time `gorm:"default:null"`
	UserId      *uint      `gorm:"default:null;foreignKey:Id;references:Users"`
	User        Users      `gorm:"foreignKey:UserId"`
	Password    *string    `gorm:"default:null"`
	CreatedAt   time.Time  `gorm:"not null"`
	UpdatedAt   time.Time  `gorm:"not null"`
	DeletedAt   *time.Time `gorm:"default:null"`
	ExpiresAt   *time.Time `gorm:"default:null"`
}

type Users struct {
	Id        uint       `gorm:"primaryKey"`
	Email     string     `gorm:"unique;not null"`
	Name      *string    `gorm:"default:null"`
	ApiKey    string     `gorm:"unique;not null"`
	Tier      string     `gorm:"default:hobby"`
	CreatedAt time.Time  `gorm:"not null"`
	UpdatedAt time.Time  `gorm:"not null"`
	DeletedAt *time.Time `gorm:"default:null"`
}

type LogRequests struct {
	Id        uint       `gorm:"primaryKey"`
	Timestamp time.Time  `gorm:"not null"`
	Method    string     `gorm:"not null"`
	Url       string     `gorm:"not null"`
	UserAgent string     `gorm:"not null"`
	IpAddress string     `gorm:"not null"`
	CreatedAt time.Time  `gorm:"not null"`
	UpdatedAt time.Time  `gorm:"not null"`
	DeletedAt *time.Time `gorm:"default:null"`
}
