package record

func init() {
	err := initConfig()
	if nil != err {
		panic(err)
	}

	err = initLog()
	if nil != err {
		panic(err)
	}

	err = initDB()
	if nil != err {
		panic(err)
	}

	err = initBlockPool()
	if nil != err {
		panic(err)
	}

	err = initStorage()
	if nil != err {
		panic(err)
	}
}
