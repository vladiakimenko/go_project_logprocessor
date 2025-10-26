package bootstrap

import (
	"encoding/json"
	"os"
)

const configPath = "config.json"

type config struct {
	Core struct {
		FilePath        string `json:"filepath"`
		WorkersNumber   int    `json:"workers"`
		TopValuesNumber int    `json:"tops"`
	} `json:"core"`
	Logging struct {
		Json  bool   `json:"json"`
		Level string `json:"level"`
	} `json:"logging"`
}

var Settings config

func loadConfig() error {
	file, err := os.Open(configPath)
	if err != nil {
		return err
	}
	defer file.Close()

	decoder := json.NewDecoder(file)
	if err := decoder.Decode(&Settings); err != nil {
		return err
	}
	return nil
}
