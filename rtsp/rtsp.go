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

	err = initDB()
	if nil != err {
		log.Panic(err)
	}

	err = initServer()
	if nil != err {
		log.Panic(err)
	}

	err = initVOD()
	if nil != err {
		log.Panic(err)
	}
}
