package record_test

import (
	"testing"

	"github.com/EasyDarwin/EasyDarwin/rtsp/record"
	"github.com/stretchr/testify/assert"
)

func TestTaskCURD(t *testing.T) {
	assert := assert.New(t)

	task := &record.Task{
		ID:   "test01",
		Path: "/test01",
	}

	err := record.AddTask(task)
	assert.Nil(err)

	task, err = record.GetTask("test02")
	assert.Nil(err)
	assert.Nil(task)

	task, err = record.GetTask("test01")
	assert.Nil(err)
	assert.NotNil(task)
	assert.Equal("test01", task.ID)
	assert.Equal("/test01", task.Path)

	tasks := record.GetAllTasks()
	assert.Equal(1, len(tasks))
	assert.Equal("test01", tasks[0].ID)
	assert.Equal("/test01", tasks[0].Path)

	err = record.RemoveTask("test01")
	assert.Nil(err)
	task, err = record.GetTask("test01")
	assert.Nil(err)
	assert.Nil(task)
}
