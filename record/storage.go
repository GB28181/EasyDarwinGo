package record

import (
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/EasyDarwin/EasyDarwin/models"
	"github.com/ncw/directio"

	"github.com/sirupsen/logrus"
)

// Storage impl in file
type Storage struct {
	currentPathChannel chan string
	writeQueue         chan *models.Block
}

var defaultStorage *Storage

func initStorage() error {
	defaultStorage = &Storage{
		currentPathChannel: make(chan string, 1),
		writeQueue:         make(chan *models.Block, config.Record.WriteQueueLength),
	}

	go defaultStorage.putStoragePath()
	go defaultStorage.storeBlock()

	return nil
}

func (storage *Storage) getStrogaePath() string {
	return <-storage.currentPathChannel
}

func (storage *Storage) scanSpace() (string, error) {
	maxFreePath := ""
	maxFree := uint64(0)

	for _, storagePath := range config.Record.StoragePath {
		stat, err := DiskUsage(storagePath)
		if nil != err {
			log.WithFields(logrus.Fields{
				"error": err,
				"path":  storagePath,
			}).Error("Scan storage")
			continue
		}
		log.WithFields(logrus.Fields{
			"all":  stat.All,
			"free": stat.Free,
			"path": storagePath,
		}).Info("disk state")
		if maxFree < stat.Free {
			maxFree = stat.Free
			maxFreePath = storagePath
		}
	}

	if 0 == maxFree {
		return "", ErrorNoneStoragePathAvaiable
	}
	log.Infof("Using storage path[%s]", maxFreePath)

	return maxFreePath, nil
}

func (storage *Storage) putStoragePath() {
	var currentPath string

	newPath, err := storage.scanSpace()
	if nil != err {
		panic(err)
	}
	currentPath = newPath

	ticker := time.NewTicker(time.Second * time.Duration(config.Record.StorageScanInterval))
	for {
		select {
		case <-ticker.C:
			{
				newPath, err = storage.scanSpace()
				if nil != err {
					log.WithField("error", err).Error("Storage")
				}
				currentPath = newPath
			}
		case storage.currentPathChannel <- currentPath:
		}
	}
}

// InsertBlock into default storage
func InsertBlock(te *models.TaskExecute, block *models.Block) error {
	// generate block ID
	blockID, err := te.GenerateBlockID()
	if nil != err {
		return err
	}

	// update TaskExecute state
	te.EndTime = block.EndTime

	block.ID = blockID
	block.TaskExecute = te

	select {
	case defaultStorage.writeQueue <- block:
		return nil
	default:
		log.Error("Stroage write queue full")
	}

	return nil
}

func blockPathes(rootPath string, block *models.Block) []string {
	bytes := make([]byte, 8)
	binary.BigEndian.PutUint64(bytes, uint64(block.ID))
	dirs := make([]string, 11)
	for i, b := range bytes {
		dirs[i+3] = fmt.Sprintf("%d", b)
	}
	dirs[0] = rootPath
	dirs[1] = block.TaskExecute.TaskID
	dirs[2] = fmt.Sprintf("%d", block.TaskExecute.ID)

	return dirs
}

func (storage *Storage) storeBlock() {
	for block := range storage.writeQueue {
		// mkdir
		pathes := blockPathes(storage.getStrogaePath(), block)
		log.Debugf("Writing block[%s]", filepath.Join(pathes...))
		dirPath := filepath.Join(pathes[:len(pathes)-1]...)
		err := os.MkdirAll(dirPath, os.ModePerm)
		if nil != err {
			log.WithField("error", err).Error("Mkdir of block")
			continue
		}
		// open block file
		blockPath := filepath.Join(dirPath, pathes[len(pathes)-1])
		{
			blockFile, err := directio.OpenFile(blockPath, os.O_WRONLY|os.O_CREATE, os.ModePerm)
			if nil != err {
				log.WithField("error", err).Error("Openfile of block")
				continue
			}
			defer blockFile.Close()
			// write block
			l, err := blockFile.Write(block.Data)
			if nil != err {
				log.WithField("error", err).Error("Write of block")
				continue
			}

			if len(block.Data) != l {
				log.WithFields(logrus.Fields{
					"actual length": l,
					"want length":   len(block.Data),
				}).Error("Write of block")
				continue
			}
		}

		block.Path = blockPath

		if err = models.AddBlockIndex(block); nil != err {
			log.WithField("error", err).Error("Add store index")
			continue
		}
		if models.UpdateExecuteTaskEndTime(block.TaskExecute, block.EndTime); nil != err {
			log.WithField("error", err).Error("Add store index")
			continue
		}

		// recycle block
		RecycleBlock(block)
	}
}

// ReadBlockData from storage
func ReadBlockData(block *models.Block) error {
	file, err := directio.OpenFile(block.Path, os.O_RDONLY, os.ModePerm)
	if nil != err {
		return err
	}

	l, err := io.ReadFull(file, block.Data)
	if nil != err {
		return err
	}

	if l != BlockSize() {
		return ErrorReadFull
	}

	return nil
}
