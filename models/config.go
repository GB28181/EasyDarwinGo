package models

import (
	"github.com/go-ini/ini"
)

type ConfigHTTP struct {
	DefaultUsername string `ini:"default_username"`
	DefaultPassword string `init:"default_password"`
}

type ConfigDatabase struct {
	Type string `ini:"type"`
	URL  string `ini:"url"`
}

type Config struct {
	HTTP     ConfigHTTP     `ini:"http"`
	Database ConfigDatabase `ini:"database"`
}

var config *Config

func init() {
	config = &Config{
		HTTP: ConfigHTTP{
			DefaultUsername: "admin",
			DefaultPassword: "admin",
		},
		Database: ConfigDatabase{
			Type: "mysql",
			URL:  "",
		},
	}
	if err := ini.MapTo(config, "./easydarwin.ini"); nil != err {
		panic(err)
	}
}
