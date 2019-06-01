package record

import (
	"container/list"

	"github.com/EasyDarwin/EasyDarwin/utils"
	"github.com/go-redis/redis"
	proto "github.com/golang/protobuf/proto"
	"github.com/ncw/directio"
)

var blockDataPool = list.New()
var blockDataPoolLocker = &utils.SpinLock{}

func initBlockPool() error {
	return nil
}

// NewBlock returns
func NewBlock() (block *Block) {
	block = &Block{}
	blockDataPoolLocker.Lock()
	if blockDataPool.Len() > 0 {
		el := blockDataPool.Front()
		block.Data = el.Value.([]byte)
		blockDataPool.Remove(el)
	}
	blockDataPoolLocker.Unlock()

	if nil == block.Data {
		block.Data = directio.AlignedBlock(config.Record.BlockSize)
	}

	return
}

// RecycleBlock to use, !IMPORTANT: make sure block is not in use
func RecycleBlock(block *Block) {
	blockDataPoolLocker.Lock()
	if blockDataPool.Len() <= 16 {
		blockDataPool.PushBack(block.Data)
	}
	blockDataPoolLocker.Unlock()
}

// AddBlockIndex to store
func AddBlockIndex(block *Block) error {
	plainBlock := *block
	plainBlock.TaskExecute = nil
	plainBlock.Data = nil

	bytes, err := proto.Marshal(&plainBlock)
	if nil != err {
		log.Errorf("Marshal task [%v]", err)
		return err
	}

	cmd := db.ZAdd(
		block.TaskExecute.getTaskExecuteBlockTimeKey(),
		redis.Z{
			Score:  float64(block.StartTime),
			Member: bytes,
		},
	)

	if nil != cmd.Err() {
		log.WithField("cmd", cmd.Args()).Error("redis")
		return cmd.Err()
	}

	return nil
}
