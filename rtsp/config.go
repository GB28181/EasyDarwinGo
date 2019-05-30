package rtsp

import "github.com/go-ini/ini"

type ConfigRTSP struct {
	NetworkBuffer       int `ini:"network_buffer"`
	Timeout             int `ini:"timeout"`
	AuthorizationEnable int `ini:"authorization_enable"`
	CloseOld            int `ini:"close_old"`
	GopCacheEnable      int `ini:"gop_cache_enable"`
	Port                int `ini:"port"`
}

type ConfigRecord struct {
	StoragePath        []string `ini:"storage_path"`
	ReceiveQueueLength int      `ini:"receive_queue_length"`
	BlockSize          int      `ini:"block_size"`
}

type ConfigLog struct {
	Level string `ini:"level"`
}

type ConfigPlayer struct {
	SendQueueLength int `ini:"send_queue_length"`
}

// Config of rtsp server
type Config struct {
	Record ConfigRecord `ini:"record"`
	RTSP   ConfigRTSP   `ini:"rtsp"`
	Log    ConfigLog    `ini:"log"`
	Player ConfigPlayer `ini:"player"`
}

var config *Config

func initConfig() error {
	config = &Config{
		RTSP: ConfigRTSP{
			NetworkBuffer:       204800,
			Timeout:             5 * 1000,
			AuthorizationEnable: 0,
			CloseOld:            0,
			GopCacheEnable:      0,
			Port:                554,
		},
		Player: ConfigPlayer{
			SendQueueLength: 128,
		},
		Record: ConfigRecord{
			ReceiveQueueLength: 128,
			BlockSize:          2 * 1024 * 1024,
		},
	}
	return ini.MapTo(config, "./easydarwin.ini")
}
