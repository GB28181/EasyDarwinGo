package rtsp

import "fmt"

func init() {
	var err error

	fmt.Println("RTSP config init success")

	err = initConfig()
	if nil != err {
		panic(err)
	}

	fmt.Println("RTSP config init success")

	err = initLog()
	if nil != err {
		panic(err)
	}

	err = initStorage()
	if nil != err {
		panic(err)
	}

	err = initServer()
	if nil != err {
		panic(err)
	}
}
