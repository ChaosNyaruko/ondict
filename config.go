package main

import (
	"encoding/json"
	"errors"
	"log"
	"os"
	"path/filepath"
)

type Config struct {
	Dicts []string `json:"dicts"`
}

func loadConfig() error { // TODO: more configurations, such as default engine.
	data, err := os.ReadFile(filepath.Join(dataPath, "config.json"))
	if err != nil && errors.Is(err, os.ErrNotExist) {
		log.Printf("load config file err: %v, default settings are used.", err)
		return err
	}
	c := Config{}
	if err := json.Unmarshal(data, &c); err != nil {
		log.Printf("bad json unmarshal: %v, default settings are used.", err)
		return err
	}
	if len(c.Dicts) == 0 {
		return nil
	}
	globalDict.mdxFile = c.Dicts[0] + ".json"
	globalDict.mdxCss = c.Dicts[0] + ".css"
	log.Printf("get global dicts: %v", globalDict)
	return nil
}
