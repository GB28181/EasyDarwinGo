package models

import (
	"github.com/go-redis/redis"
)

var db *redis.Client

func initDB() error {
	db = redis.NewClient(&redis.Options{
		Addr:     config.Redis.Host,
		Password: config.Redis.Password,
		DB:       config.Redis.DB,
	})
	return nil
}
