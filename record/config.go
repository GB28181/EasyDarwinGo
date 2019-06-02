package record

import (
	"fmt"

	"github.com/go-ini/ini"
	"github.com/ncw/directio"
)

// ConfigRecord struct
type ConfigRecord struct {
	RedisAddr           string   `ini:"redis_addr"`
	RedisPassword       string   `ini:"redis_password"`
	RedisDB             int      `ini:"redis_db"`
	StoragePath         []string `ini:"storage_path"`
	StorageScanInterval int      `ini:"storage_scan_internal"`
	WriteQueueLength    int      `ini:"write_queue_length"`
	BlockSize           int      `ini:"block_size"`
}

// ConfigLog struct
type ConfigLog struct {
	Level string `ini:"level"`
}

// Config of rtsp server
type Config struct {
	Record ConfigRecord `ini:"record"`
	Log    ConfigLog    `ini:"log"`
}

var config *Config

func initConfig() error {
	config = &Config{
		Record: ConfigRecord{
			RedisAddr:           "localhost:6379",
			RedisPassword:       "",
			RedisDB:             0,
			StorageScanInterval: 120,
			WriteQueueLength:    256,
			BlockSize:           2 * 1024 * 1024,
		},
	}
	err := ini.MapTo(config, "./easydarwin.ini")
	if nil != err {
		return err
	}

	if config.Record.BlockSize%directio.BlockSize != 0 {
		return fmt.Errorf("block size should be multi of %d", directio.BlockSize)
	}

	return nil
}
