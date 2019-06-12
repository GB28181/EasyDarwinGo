package models

import "github.com/go-ini/ini"

type ConfigHTTP struct {
	DefaultUsername string `ini:"default_username"`
	DefaultPassword string `init:"default_password"`
}

type ConfigLog struct {
	Level string `ini:"level"`
}

type ConfigRedis struct {
	Host     string `ini:"host"`
	Password string `ini:"password"`
	DB       int    `ini:"db"`
}

type Config struct {
	HTTP  ConfigHTTP  `ini:"http"`
	Redis ConfigRedis `ini:"redis"`
	Log   ConfigLog   `ini:"log"`
}

var config *Config

func initConfig() error {
	config = &Config{
		HTTP: ConfigHTTP{
			DefaultUsername: "admin",
			DefaultPassword: "admin",
		},
		Redis: ConfigRedis{
			Host:     "localhost:6379",
			Password: "",
			DB:       0,
		},
		Log: ConfigLog{
			Level: "info",
		},
	}
	return ini.MapTo(config, "./easydarwin.ini")
}
