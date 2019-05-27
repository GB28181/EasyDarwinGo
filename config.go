package main

import "github.com/go-ini/ini"

type ConfigHTTP struct {
	Port int `ini:"port"`
}

type ConfigService struct {
	Name        string `ini:"name"`
	DisplayName string `ini:"display_name"`
	Description string `ini:"description"`
}

type Config struct {
	HTTP    ConfigHTTP    `ini:"http"`
	Service ConfigService `ini:"service"`
}

var config *Config

func init() {
	config = &Config{
		HTTP: ConfigHTTP{
			Port: 10008,
		},
		Service: ConfigService{
			Name:        "EasyDarwin",
			DisplayName: "EasyDarwin",
			Description: "A RTSP server",
		},
	}
	if err := ini.MapTo(config, "./easydarwin.ini"); nil != err {
		panic(err)
	}
}
