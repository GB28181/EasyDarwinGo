package routers

func init() {
	err := initConfig()
	if nil != err {
		panic(err)
	}

	err = initLog()
	if nil != err {
		panic(err)
	}
}
