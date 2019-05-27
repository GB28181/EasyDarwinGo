package rtsp

import (
	"net"
	"os"
	"path/filepath"
	"time"

	"github.com/go-redis/redis"
	"github.com/ncw/directio"
)

// TODO: storage space monitor

type _RecordStorage struct {
	redis              *redis.Client
	storagePathes      []string
	currentPathIndex   int
	storagePathChannel chan string
	// current storage file status
	fileLen int
}

var storage *_RecordStorage

func initStorage() error {
	redis := redis.NewClient(&redis.Options{
		Addr:     "localhost:6380",
		Password: "",
		DB:       0,
	})

	storage = &_RecordStorage{
		redis:              redis,
		storagePathes:      config.Record.StoragePath,
		currentPathIndex:   0,
		storagePathChannel: make(chan string),
	}

	go storage.scanFreeSpace()

	return nil
}

func (rs *_RecordStorage) scanFreeSpace() {
	ticker := time.Tick(1 * time.Minute)
	for {
		select {
		case rs.storagePathChannel <- rs.storagePathes[rs.currentPathIndex]:
		case <-ticker:
			max := uint64(0)
			maxIndex := -1
			for index, storagePath := range rs.storagePathes {
				stats, err := DiskUsage(storagePath)
				if err != nil {
					// TODO: log error
					continue
				}
				if stats.Free > max {
					max = stats.Free
					maxIndex = index
				}
			}
			if maxIndex >= 0 {
				rs.currentPathIndex = maxIndex
			}
		}
	}
}

func (rs *_RecordStorage) generateFile() (file *os.File, err error) {
	rootPath := <-rs.storagePathChannel

	dateDirPath := filepath.Join(rootPath, time.Now().Format("2006-01-02"))
	if err = os.MkdirAll(dateDirPath, os.ModePerm); nil != err {
		return
	}

	storageFilePath := filepath.Join(dateDirPath, time.Now().Format("15-04-05"))
	file, err = directio.OpenFile(storageFilePath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, os.ModePerm)

	return
}

func (rs *_RecordStorage) Insert(taskID string, start time.Time, buffers net.Buffers) error {
	return nil
}
