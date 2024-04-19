package main

import (
	"context"
	"github.com/redis/go-redis/v9"
	"log"
)

// opens a connection to the Redis database
func connectToRedis() *redis.Client {
	// Connect to KeyDB
	db := redis.NewClient(&redis.Options{
		Addr:     getEnvVar("KEY_DB_HOST") + ":" + getEnvVar("KEY_DB_PORT"),
		Password: getEnvVar("KEY_DB_PASS"),
		DB:       0,
	})

	return db
}

func getFromRedis(key string) string {
	db := connectToRedis()
	val, err := db.Get(context.Background(), key).Result()
	if err != nil {
		log.Println("Error getting value from Redis: ", err)
	}
	return val
}

func setToRedis(key string, value string) {
	db := connectToRedis()
	err := db.Set(context.Background(), key, value, 0).Err()
	if err != nil {
		log.Println("Error setting value in Redis: ", err)
	}
}
