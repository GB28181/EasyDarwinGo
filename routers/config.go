package routers

import "github.com/go-ini/ini"

type ConfigHTTP struct {
	Static          string `ini:"static"`
	TokenTimeout    int    `ini:"token_timeout"`
	DefaultUsername string `ini:"default_username"`
	DefaultPassword string `init:"default_password"`
}

type ConfigLog struct {
	Level string `ini:"level"`
}

type Config struct {
	HTTP ConfigHTTP `ini:"http"`
	Log  ConfigLog  `ini:"log"`
}

var config *Config

func initConfig() error {
	config = &Config{
		HTTP: ConfigHTTP{
			Static:          "./www",
			TokenTimeout:    7 * 86400,
			DefaultUsername: "admin",
			DefaultPassword: "admin",
		},
		Log: ConfigLog{
			Level: "info",
		},
	}
	return ini.MapTo(config, "./easydarwin.ini")
}
