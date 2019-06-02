package record

import (
	"container/list"
	"fmt"
	"strconv"

	"github.com/EasyDarwin/EasyDarwin/utils"
	"github.com/go-redis/redis"
	proto "github.com/golang/protobuf/proto"
	"github.com/ncw/directio"
)

var blockPool = list.New()
var blockPoolLocker = &utils.SpinLock{}

func initBlockPool() error {
	return nil
}

// BlockSize is a contant
func BlockSize() int {
	return config.Record.BlockSize
}

var zeroBlock Block

// AssignBlockButData tool
func AssignBlockButData(dst, src *Block) {
	data := dst.Data
	*dst = *src
	dst.Data = data
}

// NewBlock returns
func NewBlock() (block *Block) {
	blockPoolLocker.Lock()
	if blockPool.Len() > 0 {
		el := blockPool.Front()
		block = el.Value.(*Block)
		blockPool.Remove(el)
	}
	blockPoolLocker.Unlock()

	if nil == block {
		block = &Block{}
		block.Data = directio.AlignedBlock(BlockSize())
	}

	return
}

// NewEmptyBlock without alloc Data
func NewEmptyBlock() *Block {
	return &Block{}
}

// RecycleBlock to use, !IMPORTANT: make sure block is not in use
func RecycleBlock(block *Block) {
	if nil == block.Data {
		return
	}

	blockPoolLocker.Lock()
	if blockPool.Len() <= 16 {
		AssignBlockButData(block, &zeroBlock)
		blockPool.PushBack(block)
	}
	blockPoolLocker.Unlock()
}

func (block *Block) getBlockKey() string {
	return fmt.Sprintf(
		"%s:%d:%d:teb",
		block.TaskExecute.TaskID,
		block.TaskExecute.ID,
		block.ID,
	)
}

// AddBlockIndex to store
func AddBlockIndex(block *Block) error {
	// add block index
	if cmd := db.ZAdd(
		block.TaskExecute.getTaskExecuteBlockTimeKey(),
		redis.Z{
			Score:  float64(block.StartTime),
			Member: block.ID,
		},
	); nil != cmd.Err() {
		log.WithField("cmd", cmd.Args()).Error("redis")
		return cmd.Err()
	}

	// store block info

	// shrink block info
	plainBlock := *block
	plainBlock.Data = nil
	plainBlock.TaskExecute = &TaskExecute{
		ID:     plainBlock.TaskExecute.ID,
		TaskID: plainBlock.TaskExecute.TaskID,
	}

	bytes, err := proto.Marshal(&plainBlock)
	if nil != err {
		log.Errorf("Marshal task [%v]", err)
		return err
	}

	if cmd := db.Set(plainBlock.getBlockKey(), bytes, 0); nil != cmd.Err() {
		log.WithField("cmd", cmd.Args()).Error("redis")
		return cmd.Err()
	}

	return nil
}

// GetBlockByID according taskID executeID and blockID
func GetBlockByID(block *Block) error {
	cmd := db.Get(block.getBlockKey())
	if bytes, err := cmd.Bytes(); nil != err {
		log.WithField("cmd", cmd.Args()).Error("redis")
		return cmd.Err()
	} else if err = proto.Unmarshal(bytes, block); nil != err {
		log.WithField("bytes", bytes).Error("protobuf Unmarshal")
		return err
	}

	return nil
}

// GetBlockByTime according taskID executeID and time
func GetBlockByTime(block *Block, time int64) error {
	cmd := db.ZRangeByScore(
		block.TaskExecute.getTaskExecuteBlockTimeKey(),
		redis.ZRangeBy{
			Min:    fmt.Sprintf("%d", time),
			Max:    "+inf",
			Offset: 0,
			Count:  1,
		},
	)
	if nil != cmd.Err() {
		log.WithField("cmd", cmd.Args()).Error("redis")
		return cmd.Err()
	}

	if len(cmd.Val()) == 0 {
		return ErrorBlockNotFount
	}

	blockID, err := strconv.ParseInt(cmd.Val()[0], 10, 63)
	if nil != err {
		return ErrorBlockMalformed
	}

	block.ID = blockID

	return GetBlockByID(block)
}
