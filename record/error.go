package record

import "errors"

// errors
var (
	ErrorNoneStoragePathAvaiable = errors.New("None storage path avaiable")
	ErrorDB                      = errors.New("DB error")
	ErrorReadFull                = errors.New("Not read full")
	ErrorBlockNotFount           = errors.New("Not found block")
	ErrorBlockMalformed          = errors.New("Block bytes malformed")
	ErrorTaskExecuteMalformed    = errors.New("TaskExecute bytes malformed")
	ErrorDBOperation             = errors.New("DB operation error")
)
