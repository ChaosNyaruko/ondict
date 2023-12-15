package sources

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

func LoadConfig() error {
	data, err := os.ReadFile(filepath.Join(DataPath, "config.json"))
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
	GlobalDict.MdxFile = c.Dicts[0]
	GlobalDict.MdxCss = c.Dicts[0] + ".css"
	log.Printf("get global dicts: %v", GlobalDict)
	return nil
}
