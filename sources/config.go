package sources

import (
	"encoding/json"
	"errors"
	"log"
	"os"
	"path/filepath"

	"github.com/ChaosNyaruko/ondict/util"
)

type Config struct {
	Dicts []string `json:"dicts"`
}

func LoadConfig() error {
	data, err := os.ReadFile(filepath.Join(util.ConfigPath(), "config.json"))
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
	for _, name := range c.Dicts {
		dict := &MdxDict{}
		dict.MdxFile = filepath.Join(util.DictsPath(), name)
		dict.MdxCss = filepath.Join(util.DictsPath(), name+".css")
		log.Printf("get global dict: %v", dict.MdxFile)
		*G = append(*G, dict)
	}
	log.Printf("get global dicts: %v", G)
	return nil
}
