package record

import (
	"container/list"

	"github.com/EasyDarwin/EasyDarwin/models"
	"github.com/EasyDarwin/EasyDarwin/utils"
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

var zeroBlock models.Block

// AssignBlockButData tool
func AssignBlockButData(dst, src *models.Block) {
	data := dst.Data
	*dst = *src
	dst.Data = data
}

// NewBlock returns
func NewBlock() (block *models.Block) {
	blockPoolLocker.Lock()
	if blockPool.Len() > 0 {
		el := blockPool.Front()
		block = el.Value.(*models.Block)
		blockPool.Remove(el)
	}
	blockPoolLocker.Unlock()

	if nil == block {
		block = &models.Block{}
		block.Data = directio.AlignedBlock(BlockSize())
	}

	return
}

// NewEmptyBlock without alloc Data
func NewEmptyBlock() *models.Block {
	return &models.Block{}
}

// RecycleBlock to use, !IMPORTANT: make sure block is not in use
func RecycleBlock(block *models.Block) {
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
