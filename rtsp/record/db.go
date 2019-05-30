package record

import (
	"github.com/go-redis/redis"
)

var db *redis.Client

func initDB() error {
	db = redis.NewClient(&redis.Options{
		Addr:     config.Record.RedisAddr,
		Password: config.Record.RedisPassword,
		DB:       0,
	})
	return nil
}
