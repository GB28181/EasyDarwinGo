package routers

import "github.com/go-ini/ini"

type ConfigHTTP struct {
	Static          string `ini:"static"`
	TokenTimeout    int    `ini:"token_timeout"`
	DefaultUsername string `ini:"default_username"`
	DefaultPassword string `init:"default_password"`
}

type Config struct {
	HTTP ConfigHTTP `ini:"http"`
}

var config *Config

func init() {
	config = &Config{
		HTTP: ConfigHTTP{
			Static:          "./www",
			TokenTimeout:    7 * 86400,
			DefaultUsername: "admin",
			DefaultPassword: "admin",
		},
	}
	if err := ini.MapTo(config, "./easydarwin.ini"); nil != err {
		panic(err)
	}
}
