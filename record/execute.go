package record

import (
	"fmt"
	"time"

	"github.com/go-redis/redis"
	proto "github.com/golang/protobuf/proto"
	"github.com/sirupsen/logrus"
)

// AddExecuteTask of record
func AddExecuteTask(task *Task, te *TaskExecute) error {
	bytes, err := proto.Marshal(te)
	if nil != err {
		log.Errorf("Marshal task [%v]", err)
		return err
	}

	_, err = db.ZAdd(
		task.getExecuteKey(),
		redis.Z{
			Score:  float64(te.StartTime),
			Member: te.ID,
		}).Result()

	if nil != err {
		log.Errorf("DB zadd [%v]", err)
		return err
	}

	// TODO: speed up DB operate in parallel mode
	_, err = db.Set(te.getTaskExecuteKey(), bytes, 0).Result()

	if nil != err {
		log.Errorf("DB zadd [%v]", err)
		return err
	}

	_, err = db.Set(te.getTaskExecuteStartTimeKey(), te.StartTime, 0).Result()
	if nil != err {
		log.Errorf("DB set [%v]", err)
		return err
	}

	_, err = db.Set(te.getTaskExecuteEndTimeKey(), te.EndTime, 0).Result()
	if nil != err {
		log.Errorf("DB set [%v]", err)
		return err
	}

	return nil
}

// GetExecuteTask according to taskID and executeID
func GetExecuteTask(te *TaskExecute) error {
	var bytes []byte

	if cmd := db.Get(te.getTaskExecuteKey()); nil != cmd.Err() {
		log.WithError(cmd.Err()).WithField("cmd", cmd.Args()).Error("redis")
		return ErrorDBOperation
	} else {
		bytes, _ = cmd.Bytes()
	}

	err := proto.Unmarshal(bytes, te)
	if nil != err {
		log.WithError(err).Error("proto.Unmarshal")
		return ErrorTaskExecuteMalformed
	}

	startTime, err := db.Get(te.getTaskExecuteStartTimeKey()).Int64()
	if nil != err {
		log.Errorf("DB set [%v]", err)
		return err
	}
	te.StartTime = startTime

	endTime, err := db.Get(te.getTaskExecuteEndTimeKey()).Int64()
	if nil != err {
		log.Errorf("DB set [%v]", err)
		return err
	}
	te.EndTime = endTime

	return nil
}

// UpdateExecuteTaskEndTime to store
func UpdateExecuteTaskEndTime(te *TaskExecute, endTime int64) error {
	// TODO: using zset store execute end time
	cmd := db.Set(te.getTaskExecuteEndTimeKey(), endTime, 0)
	if nil != cmd.Err() {
		log.WithField("cmd", cmd.Args()).Error("redis")
		return cmd.Err()
	}

	return nil
}

// ExecuteTask returns
func ExecuteTask(task *Task, SDPRaw string) (*TaskExecute, error) {
	executeID, err := db.Incr(task.getExecuteSeqKey()).Result()
	if nil != err {
		log.Errorf("DB incr [%v]", err)
		return nil, err
	}

	te := &TaskExecute{
		ID:        executeID,
		TaskID:    task.ID,
		SDPRaw:    SDPRaw,
		StartTime: time.Now().Unix(),
	}

	err = AddExecuteTask(task, te)
	if nil != err {
		return nil, err
	}

	return te, nil
}

func (te *TaskExecute) getTaskExecuteKey() string {
	return fmt.Sprintf("%s:%d:te", te.TaskID, te.ID)
}

func (te *TaskExecute) getTaskExecuteBlockIDKey() string {
	return fmt.Sprintf("%s:%d:tebs", te.TaskID, te.ID)
}

func (te *TaskExecute) getTaskExecuteStartTimeKey() string {
	return fmt.Sprintf("%s:%d:test", te.TaskID, te.ID)
}

func (te *TaskExecute) getTaskExecuteEndTimeKey() string {
	return fmt.Sprintf("%s:%d:teet", te.TaskID, te.ID)
}

func (te *TaskExecute) getTaskExecuteBlockTimeKey() string {
	return fmt.Sprintf("%s:%d:tebt", te.TaskID, te.ID)
}

func (te *TaskExecute) getBlockID() (int64, error) {
	cmd := db.Incr(te.getTaskExecuteBlockIDKey())

	if nil != cmd.Err() {
		log.WithFields(logrus.Fields{
			"err": cmd.Err(),
			"cmd": cmd.Args(),
		}).Error("DB redis")
		return 0, ErrorDB
	}

	return cmd.Val(), nil
}

// InsertBlock to storage
func (te *TaskExecute) InsertBlock(block *Block) error {
	// generate block ID
	blockID, err := te.getBlockID()
	if nil != err {
		return err
	}

	// update TaskExecute state
	te.EndTime = block.EndTime

	block.ID = blockID
	block.TaskExecute = te

	return storage.insertBlock(block)
}
