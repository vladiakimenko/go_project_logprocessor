package bootstrap

func init() {
	if err := loadConfig(); err != nil {
		panic("failed to load config: " + err.Error())
	}

	if logger, err := getLogger(&Settings); err != nil {
		panic("failed to initialize logger: " + err.Error())
	} else {
		Logger = logger
	}
}
