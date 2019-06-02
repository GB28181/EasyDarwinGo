package record

import (
	"fmt"

	"github.com/go-redis/redis"
	proto "github.com/golang/protobuf/proto"
)

// AddTask of record
func AddTask(task *Task) error {
	bytes, err := proto.Marshal(task)
	if nil != err {
		log.Errorf("Marshal task [%v]", err)
		return err
	}
	_, err = db.HSet("task", task.ID, bytes).Result()

	if nil != err {
		log.Errorf("DB add [%v]", err)
		return err
	}

	return nil
}

// GetTask of record
func GetTask(ID string) (*Task, error) {
	var bytes string
	bytes, err := db.HGet("task", ID).Result()
	if nil != err {
		if redis.Nil == err {
			return nil, nil
		}
		log.Errorf("DB get [%v]", err)
		return nil, err
	}

	task := &Task{}
	if err := proto.Unmarshal([]byte(bytes), task); err != nil {
		log.Errorf("Unmarshal task [%v]", err)
		return nil, err
	}

	return task, nil
}

// RemoveTask of record
func RemoveTask(ID string) (err error) {
	_, err = db.HDel("task", ID).Result()
	if nil != err {
		log.Errorf("DB del [%v]", err)
		return
	}

	return
}

// GetAllTasks of record
func GetAllTasks() (tasks []*Task) {
	all := db.HGetAll("task")

	for _, bytes := range all.Val() {
		task := &Task{}
		if err := proto.Unmarshal([]byte(bytes), task); err != nil {
			log.Errorf("Unmarshal task [%v]", err)
			continue
		}
		tasks = append(tasks, task)
	}

	return
}

func (t *Task) getExecuteSeqKey() string {
	return fmt.Sprintf("%s:tes", t.ID)
}

func (t *Task) getExecuteKey() string {
	return fmt.Sprintf("%s:te", t.ID)
}
