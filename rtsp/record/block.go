package record

import (
	"sync"

	"github.com/go-redis/redis"
	proto "github.com/golang/protobuf/proto"
	"github.com/ncw/directio"
)

var blockPool *sync.Pool // optimize

func newBlock() interface{} {
	block := &Block{
		Data: directio.AlignedBlock(config.Record.BlockSize),
	}
	return block
}

func initBlockPool() error {
	blockPool = &sync.Pool{
		New: newBlock,
	}
	return nil
}

// NewBlock returns
func NewBlock() *Block {
	return blockPool.Get().(*Block)
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
