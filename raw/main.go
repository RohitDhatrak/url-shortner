package m1a1

import (
	"context"
	"fmt"

	"time"

	"crypto/sha256"
	"encoding/base64"
	"errors"

	"github.com/google/uuid"
	"go.mongodb.org/mongo-driver/bson"
	"golang.org/x/exp/rand"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"

	"log"

	"github.com/qiniu/qmgo"
	"github.com/qiniu/qmgo/options"
)

const MAX_RETRIES = 3
const NORMAL_SHORT_CODE_LENGTH = 8
const USE_NO_SQL = false

var db *gorm.DB
var client *qmgo.Client

func main() {
	db = initDB()
	// client = initMongoDB()
	const NO_OF_ENTRIES = 10_00_000
	const NO_OF_TIMES_QUERY = 10_000
	startedTime := time.Now().Format("15:04:05")

	addNEntries(NO_OF_ENTRIES)
	// queryNTimes(NO_OF_TIMES_QUERY)

	fmt.Println("Time started:", startedTime)
	fmt.Println("Time ended:", time.Now().Format("15:04:05"))
	// defer client.Close(context.Background())
}

func initDB() *gorm.DB {
	dsn := "user=postgres dbname=vyson_db port=5433 sslmode=disable"
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		panic("Failed to connect database")
	}

	return db
}

func initMongoDB() *qmgo.Client {
	client, err := qmgo.NewClient(context.TODO(), &qmgo.Config{Uri: "mongodb://localhost:27017"})
	if err != nil {
		log.Fatal(err)
	}

	collection := client.Database("admin").Collection("url_shortners")
	collection.CreateOneIndex(context.Background(), options.IndexModel{Key: []string{"short_code"}})

	return client
}

func queryNTimes(noOfTimesToQuery int) {
	shortCodes := []string{"OEWpcwvi", "ST2Xo4eP", "mc24YGya", "yHkf4oXB", "AwibCalY"}
	mongoShortCodes := []string{"a5E5IrqQ", "VWPMg1Uj", "5wXp3ZKE", "TNEa33ij", "epr3Javk"}

	for i := 0; i < noOfTimesToQuery; i++ {
		if USE_NO_SQL {
			models := []UrlShortenerMongoDb{}
			filter := bson.M{"short_code": bson.M{"$in": mongoShortCodes}}

			collection := client.Database("admin").Collection("url_shortners")
			err := collection.Find(context.TODO(), filter).All(&models)
			if err != nil {
				panic(err)
			}
		} else {
			models := []UrlShortener{}
			result := db.Where("short_code IN ?", shortCodes).Find(&models)
			if result.Error != nil {
				panic(result.Error)
			}
		}
	}
}

func addNEntries(noOfEntries int) {
	for i := 0; i < noOfEntries; i++ {
		originalUrl := fmt.Sprintf("https://www.example.com/%s", uuid.New().String())
		createShortUrl(originalUrl)
	}
}

func createShortUrl(originalUrl string) {
	shortCode := hashedUrl(originalUrl, 0)
	createShortUrlWithRetry(originalUrl, shortCode, MAX_RETRIES)
}

func hashedUrl(originalUrl string, additionalLength uint) string {
	HASH_TRIM_LENGTH := NORMAL_SHORT_CODE_LENGTH + additionalLength
	hash := sha256.Sum256([]byte(originalUrl))
	shortCode := base64.StdEncoding.EncodeToString(hash[:])

	return shortCode[:HASH_TRIM_LENGTH]
}

func createShortUrlWithRetry(ogUrl, shortCode string, retryCount uint) {
	shortCodeExists := doesShortCodeExist(shortCode)
	if shortCodeExists {
		if retryCount > 0 {
			newShortCode := hashedUrl(ogUrl+uuid.New().String(), MAX_RETRIES-retryCount)
			createShortUrlWithRetry(ogUrl, newShortCode, retryCount-1)
		} else {
			errMsg := "Error creating short url, max retry count exceded " + ogUrl
			panic(errMsg)
		}
		return
	}

	start := time.Date(2024, time.January, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2025, time.February, 1, 23, 59, 59, 0, time.UTC)

	randomTime := randomTimestamp(start, end)

	if USE_NO_SQL {
		collection := client.Database("admin").Collection("url_shortners")

		collection.InsertOne(context.TODO(),
			UrlShortenerMongoDb{
				OriginalUrl: ogUrl,
				ShortCode:   shortCode,
				// CreatedAt:   randomTime,
				// UpdatedAt:   randomTime,
			},
		)

	} else {
		result := db.Create(&UrlShortener{
			OriginalUrl: ogUrl,
			ShortCode:   shortCode,
			CreatedAt:   randomTime,
			UpdatedAt:   randomTime,
		})

		if result.Error != nil {
			panic(result.Error)
		}
	}
}

func randomTimestamp(min, max time.Time) time.Time {
	minUnix := min.Unix()
	maxUnix := max.Unix()

	delta := maxUnix - minUnix

	randomSec := rand.Int63n(delta) + minUnix

	return time.Unix(randomSec, 0)
}

func doesShortCodeExist(shortCode string) bool {
	if USE_NO_SQL {
		model := UrlShortenerMongoDb{}
		collection := client.Database("admin").Collection("url_shortners")
		err := collection.Find(context.TODO(), bson.M{"short_code": shortCode}).One(&model)
		if err != nil {
			if errors.Is(err, qmgo.ErrNoSuchDocuments) {
				return false
			} else {
				panic(err.Error())
			}
		}
	} else {
		model := UrlShortener{}
		result := db.Model(UrlShortener{}).First(&model, UrlShortener{ShortCode: shortCode})
		if result.Error != nil {
			if errors.Is(result.Error, gorm.ErrRecordNotFound) {
				return false
			} else {
				panic(result.Error)
			}
		}
	}

	return true
}
