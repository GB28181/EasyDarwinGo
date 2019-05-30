package rtsp

func init() {
	var err error

	err = initConfig()
	if nil != err {
		panic(err)
	}

	err = initLog()
	if nil != err {
		panic(err)
	}

	err = initServer()
	if nil != err {
		panic(err)
	}
}
